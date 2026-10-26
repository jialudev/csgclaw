package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"slices"
	"time"

	"csgclaw/internal/agentengine"
	"csgclaw/internal/agentsession"
	"csgclaw/internal/legacyimport"
	skilllocal "csgclaw/internal/skill/local"
)

func (h *Handler) writeImportError(w http.ResponseWriter, err error) {
	if errors.Is(err, legacyimport.ErrConflict) {
		controlError(w, 409, "import_conflict", "imported content conflicts with an existing file or skill")
		return
	}
	writeControlError(w, err)
}
func (h *Handler) attachLegacySession(record legacyimport.Record) error {
	if len(record.History) == 0 {
		return nil
	}
	if h.sessionBindings == nil {
		return &agentengine.TurnError{Code: agentengine.ErrorAgentUnavailable, Message: "session storage is unavailable"}
	}
	return h.sessionBindings.BindExisting(agentsession.Binding{AgentID: record.AgentID, ExternalSession: "legacy." + record.ID, ConversationKey: "import:" + record.ID, SourceBackend: record.SourceID, ReadOnly: true})
}
func (h *Handler) getLegacyImport(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	var record legacyimport.Record
	if err := h.control.store.Read(legacyimport.Key(a.ID, controlPathID(r, "import_id")), &record); err != nil {
		writeControlError(w, err)
		return
	}
	writeJSON(w, 200, record)
}
func (h *Handler) listLegacyImports(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	raws, err := h.control.store.Records()
	if err != nil {
		writeControlError(w, err)
		return
	}
	out := []legacyimport.Record{}
	for _, raw := range raws {
		var record legacyimport.Record
		if json.Unmarshal(raw, &record) == nil && record.Kind == "legacy_import" && record.AgentID == a.ID {
			record.History = nil
			out = append(out, record)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}
func (h *Handler) createLegacyImport(w http.ResponseWriter, r *http.Request) {
	a, ok := h.controlAgent(w, r)
	if !ok {
		return
	}
	var request legacyimport.Request
	if !decodeControlLimit(w, r, &request, legacyimport.MaxBody) {
		return
	}
	if err := legacyimport.Validate(request); err != nil {
		controlError(w, 422, "invalid_import", err.Error())
		return
	}
	if request.Configuration != nil {
		for _, field := range request.Configuration.FieldMask {
			switch field {
			case "instructions", "mcp_servers", "model.selector", "model.model_id", "model.reasoning_effort", "model.fast_mode":
			default:
				controlError(w, 422, "unsupported_import_field", "field cannot be imported: "+field)
				return
			}
		}
		if len(request.Configuration.FieldMask) == 0 {
			controlError(w, 400, "invalid_import", "configuration field_mask is required")
			return
		}
	}
	h.control.mu.Lock()
	defer h.control.mu.Unlock()
	a, err := h.agentEngine.Agents().Get(r.Context(), a.ID, agentengine.AgentGetOptions{})
	if err != nil {
		writeControlError(w, err)
		return
	}
	id := legacyimport.RecordID(request)
	key := legacyimport.Key(a.ID, id)
	var record legacyimport.Record
	err = h.control.store.Read(key, &record)
	if err != nil && !os.IsNotExist(err) {
		writeControlError(w, err)
		return
	}
	if err == nil {
		if record.RequestHash != legacyimport.Hash(request) {
			controlError(w, 409, "import_source_changed", "this source session was already imported with different content")
			return
		}
		if record.State == "completed" {
			if err = h.attachLegacySession(record); err != nil {
				h.writeImportError(w, err)
				return
			}
			writeJSON(w, 200, record)
			return
		}
		if record.State == "needs_review" || (record.State == "applying" && configurationRevision(a) != record.ExpectedRevision) {
			record.State = "needs_review"
			record.UpdatedAt = time.Now().UTC()
			_ = h.control.store.Write(key, record)
			controlError(w, 409, "import_needs_review", "configuration may already have been applied; inspect current state before any further import")
			return
		}
	}
	if request.ExpectedRevision != configurationRevision(a) {
		controlError(w, 409, "configuration_conflict", "configuration changed; rebase the import before retrying")
		return
	}
	if os.IsNotExist(err) {
		now := time.Now().UTC()
		record = legacyimport.Record{Kind: "legacy_import", ID: id, AgentID: a.ID, SourceID: request.SourceID, SourceSession: request.SourceSession, SourceRevision: request.SourceRevision, RequestHash: legacyimport.Hash(request), State: "staging", ExpectedRevision: request.ExpectedRevision, Files: map[string]legacyimport.FileRecord{}, Skills: map[string]string{}, History: request.History, CreatedAt: now, UpdatedAt: now}
		if err = h.control.store.Write(key, record); err != nil {
			writeControlError(w, err)
			return
		}
	}
	for _, input := range request.Files {
		if _, exists := record.Files[input.Path]; exists {
			continue
		}
		if h.workspace == nil {
			controlError(w, 503, "workspace_unavailable", "workspace is unavailable")
			return
		}
		root, err := h.workspace.WorkspaceRootByID(a.ID)
		if err != nil {
			writeControlError(w, err)
			return
		}
		content, _ := legacyimport.Decode(input)
		mediaType := input.MediaType
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		file, err := h.agentEngine.Conversations(a.ID).Files().Create(r.Context(), agentengine.FileCreateRequest{IdempotencyKey: "import:" + id + ":" + legacyimport.ID(input.Path, ""), Name: path.Base(input.Path), MIMEType: mediaType, SizeBytes: int64(len(content)), SHA256: input.SHA256}, bytes.NewReader(content))
		if err != nil {
			writeControlError(w, err)
			return
		}
		target := path.Join("imports", id, input.Path)
		if err = legacyimport.WriteOnce(root, target, content); err != nil {
			h.writeImportError(w, err)
			return
		}
		record.Files[input.Path] = legacyimport.FileRecord{Path: input.Path, WorkspacePath: target, FileID: file.ID, SHA256: input.SHA256, Size: len(content)}
		record.UpdatedAt = time.Now().UTC()
		if err = h.control.store.Write(key, record); err != nil {
			writeControlError(w, err)
			return
		}
	}
	for _, skill := range request.Skills {
		if _, exists := record.Skills[skill.Name]; exists {
			continue
		}
		rootFunc := h.control.skillRoot
		if rootFunc == nil {
			rootFunc = skilllocal.SkillsRoot
		}
		root, err := rootFunc()
		if err != nil {
			writeControlError(w, err)
			return
		}
		if err = legacyimport.InstallSkill(root, skill); err != nil {
			h.writeImportError(w, err)
			return
		}
		record.Skills[skill.Name] = skill.Name
		record.UpdatedAt = time.Now().UTC()
		if err = h.control.store.Write(key, record); err != nil {
			writeControlError(w, err)
			return
		}
	}
	if request.Configuration != nil || len(request.Skills) > 0 {
		spec := agentengine.AgentSpec{}
		fields := []string{}
		if request.Configuration != nil {
			spec = request.Configuration.Spec
			fields = append(fields, request.Configuration.FieldMask...)
		}
		names := []string{}
		if len(request.Skills) > 0 {
			spec.Skills = slices.Clone(a.Spec.Skills)
			for _, skill := range request.Skills {
				names = append(names, skill.Name)
				if !slices.Contains(spec.Skills, skill.Name) {
					spec.Skills = append(spec.Skills, skill.Name)
				}
			}
			fields = append(fields, "skills")
		}
		record.State = "applying"
		record.ExpectedRevision = request.ExpectedRevision
		record.UpdatedAt = time.Now().UTC()
		if err = h.control.store.Write(key, record); err != nil {
			writeControlError(w, err)
			return
		}
		updated, err := h.agentEngine.Agents().Update(r.Context(), a.ID, agentengine.AgentUpdateRequest{Spec: spec, FieldMask: fields, ReplaceSkills: names, ConfigurationRevision: request.ExpectedRevision, ResourceVersion: a.ResourceVersion})
		if err != nil {
			current, getErr := h.agentEngine.Agents().Get(r.Context(), a.ID, agentengine.AgentGetOptions{})
			record.State = "staging"
			if getErr != nil || configurationRevision(current) != request.ExpectedRevision {
				record.State = "needs_review"
			}
			record.UpdatedAt = time.Now().UTC()
			if saveErr := h.control.store.Write(key, record); saveErr != nil {
				writeControlError(w, saveErr)
				return
			}
			writeControlError(w, err)
			return
		}
		record.AppliedRevision = configurationRevision(updated)
		h.publishUpdatedAgentUser(serviceAgentFromEngine(updated))
	}
	record.State = "completed"
	record.UpdatedAt = time.Now().UTC()
	if err = h.control.store.Write(key, record); err != nil {
		writeControlError(w, err)
		return
	}
	if err = h.attachLegacySession(record); err != nil {
		h.writeImportError(w, err)
		return
	}
	writeJSON(w, 201, record)
}
func (h *Handler) legacyFileReferenced(agentID, fileID string) (bool, error) {
	raws, err := h.control.store.Records()
	if err != nil {
		return false, err
	}
	for _, raw := range raws {
		var record legacyimport.Record
		if json.Unmarshal(raw, &record) == nil && record.Kind == "legacy_import" && record.AgentID == agentID {
			for _, file := range record.Files {
				if file.FileID == fileID {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
