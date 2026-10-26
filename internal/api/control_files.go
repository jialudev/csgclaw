package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"csgclaw/internal/agentengine"
	"csgclaw/internal/utils/filebrowse"
)

const controlFileLimit = 32 * 1024 * 1024

func (h *Handler) createControlFile(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	// A bounded read also supports chunked upload without trusting Content-Length.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, controlFileLimit))
	if err != nil {
		controlError(w, 413, "file_too_large", "file exceeds the upload limit")
		return
	}
	mediaType := r.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	file, err := h.agentEngine.Conversations(a.ID).Files().Create(r.Context(), agentengine.FileCreateRequest{Name: r.URL.Query().Get("name"), MIMEType: mediaType, SizeBytes: int64(len(body)), SHA256: r.Header.Get("X-Content-SHA256")}, bytes.NewReader(body))
	if err != nil {
		writeControlError(w, err)
		return
	}
	writeJSON(w, 201, file)
}
func (h *Handler) getControlFile(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	file, err := h.agentEngine.Conversations(a.ID).Files().Get(r.Context(), controlPathID(r, "file_id"))
	if err != nil {
		writeControlError(w, err)
		return
	}
	defer file.Content.Close()
	w.Header().Set("Content-Type", file.Metadata.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Metadata.Name}))
	w.Header().Set("Content-Length", strconv.FormatInt(file.Metadata.SizeBytes, 10))
	w.Header().Set("ETag", `"`+file.Metadata.SHA256+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	io.Copy(w, file.Content)
}
func (h *Handler) deleteControlFile(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	id := controlPathID(r, "file_id")
	if refs, ok := h.agentEngine.(interface {
		FileReferenced(string, string) (bool, error)
	}); ok {
		used, err := refs.FileReferenced(a.ID, id)
		if err != nil {
			controlError(w, 503, "storage_unavailable", "file references could not be checked")
			return
		}
		if used {
			controlError(w, 409, "file_in_use", "file is referenced by an execution")
			return
		}
	}
	if err := h.agentEngine.Conversations(a.ID).Files().Delete(r.Context(), id); err != nil {
		writeControlError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) mutateControlWorkspace(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	if h.workspace == nil {
		controlError(w, 503, "workspace_unavailable", "workspace is unavailable")
		return
	}
	rel := r.URL.Query().Get("path")
	if rel == "" || rel == "." || path.IsAbs(rel) || strings.ContainsAny(rel, "\\\x00") || path.Clean(rel) != rel || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, ".csgclaw/") {
		controlError(w, 400, "invalid_path", "a workspace-relative file path is required")
		return
	}
	rootPath, err := h.workspace.WorkspaceRootByID(a.ID)
	if err != nil {
		writeControlError(w, err)
		return
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		writeControlError(w, err)
		return
	}
	defer root.Close()
	h.control.mu.Lock()
	defer h.control.mu.Unlock()
	// Reject symbolic links, including links that happen to stay inside the root.
	parts := strings.Split(rel, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			controlError(w, 400, "invalid_path", "symbolic links cannot be edited")
			return
		}
		if err != nil && !os.IsNotExist(err) {
			writeControlError(w, err)
			return
		}
	}
	var current []byte
	existing, err := filebrowse.OpenFile(rootPath, rel)
	if err == nil {
		current, err = io.ReadAll(io.LimitReader(existing, controlFileLimit+1))
		existing.Close()
		if len(current) > controlFileLimit {
			controlError(w, 413, "file_too_large", "existing file exceeds the write limit")
			return
		}
	}
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		writeControlError(w, err)
		return
	}
	expected := strings.Trim(r.Header.Get("If-Match"), `"`)
	sum := sha256.Sum256(current)
	revision := hex.EncodeToString(sum[:])
	if expected == "" || (exists && expected != revision) || (!exists && expected != "*") {
		controlError(w, 409, "file_conflict", "If-Match must contain the current SHA-256, or * when creating a new file")
		return
	}
	if r.Method == http.MethodDelete {
		if !exists {
			controlError(w, 404, "file_not_found", "file does not exist")
			return
		}
		if err = root.Remove(rel); err != nil {
			writeControlError(w, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	content, err := io.ReadAll(http.MaxBytesReader(w, r.Body, controlFileLimit))
	if err != nil {
		controlError(w, 413, "file_too_large", "file exceeds the write limit")
		return
	}
	if err = root.MkdirAll(path.Dir(rel), 0700); err != nil {
		writeControlError(w, err)
		return
	}
	tmp := path.Join(path.Dir(rel), ".csgclaw-write-"+newAgentSessionAPIID("file"))
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		writeControlError(w, err)
		return
	}
	defer root.Remove(tmp)
	_, err = f.Write(content)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = root.Rename(tmp, rel)
	}
	if err != nil {
		writeControlError(w, err)
		return
	}
	sum = sha256.Sum256(content)
	revision = hex.EncodeToString(sum[:])
	w.Header().Set("ETag", `"`+revision+`"`)
	writeJSON(w, 200, map[string]any{"path": rel, "sha256": revision, "size_bytes": len(content)})
}
