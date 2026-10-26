// Package legacyimport validates and stages data exported by an authorized
// legacy service. It never reads that service or executes imported code.
package legacyimport

import (
	"bytes"
	"crypto/sha256"
	"csgclaw/internal/utils/filebrowse"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"csgclaw/internal/agentengine/contract"
	skilllocal "csgclaw/internal/skill/local"
)

const MaxBody = 32 * 1024 * 1024

var ErrConflict = errors.New("import conflicts with existing content")
var sourcePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{0,127}$`)

type File struct {
	Executable bool   `json:"executable,omitempty"`
	Path       string `json:"path"`
	MediaType  string `json:"media_type"`
	SHA256     string `json:"sha256"`
	Content    string `json:"content_base64"`
}
type Skill struct {
	Name  string `json:"name"`
	Files []File `json:"files"`
}
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
type Configuration struct {
	FieldMask []string           `json:"field_mask"`
	Spec      contract.AgentSpec `json:"spec"`
}
type Request struct {
	SourceRevision   string         `json:"source_revision,omitempty"`
	SourceID         string         `json:"source_id"`
	SourceSession    string         `json:"source_session"`
	ExpectedRevision string         `json:"expected_revision"`
	Configuration    *Configuration `json:"configuration,omitempty"`
	Files            []File         `json:"files,omitempty"`
	Skills           []Skill        `json:"skills,omitempty"`
	History          []Message      `json:"history,omitempty"`
}
type FileRecord struct {
	Path          string `json:"path"`
	WorkspacePath string `json:"workspace_path"`
	FileID        string `json:"file_id"`
	SHA256        string `json:"sha256"`
	Size          int    `json:"size_bytes"`
}
type Record struct {
	SourceRevision   string                `json:"source_revision,omitempty"`
	Kind             string                `json:"kind"`
	ID               string                `json:"id"`
	AgentID          string                `json:"agent_id"`
	SourceID         string                `json:"source_id"`
	SourceSession    string                `json:"source_session"`
	RequestHash      string                `json:"request_hash"`
	State            string                `json:"state"`
	ExpectedRevision string                `json:"expected_revision"`
	AppliedRevision  string                `json:"applied_revision,omitempty"`
	Files            map[string]FileRecord `json:"files"`
	Skills           map[string]string     `json:"skills"`
	History          []Message             `json:"history,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

func ID(source, session string) string {
	sum := sha256.Sum256([]byte(source + "\x00" + session))
	return hex.EncodeToString(sum[:16])
}
func Key(agent, id string) string { return "legacy_import:" + agent + ":" + id }
func Hash(request Request) string {
	request.ExpectedRevision = ""
	raw, _ := json.Marshal(request)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func ValidPath(value string) bool {
	return value != "" && value != "." && !path.IsAbs(value) && path.Clean(value) == value && !strings.HasPrefix(value, "../") && !strings.ContainsAny(value, "\\\x00") && len(value) <= 512
}
func Decode(file File) ([]byte, error) {
	if !ValidPath(file.Path) {
		return nil, fmt.Errorf("file path is unsafe")
	}
	body, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil || len(body) > 16*1024*1024 {
		return nil, fmt.Errorf("file content is invalid or too large")
	}
	sum := sha256.Sum256(body)
	if file.SHA256 != hex.EncodeToString(sum[:]) {
		return nil, fmt.Errorf("file SHA-256 does not match")
	}
	return body, nil
}
func Validate(request Request) error {
	if !sourcePattern.MatchString(request.SourceID) || !sourcePattern.MatchString(request.SourceSession) {
		return fmt.Errorf("source_id and source_session must be path-safe identifiers")
	}
	if request.SourceRevision != "" && !sourcePattern.MatchString(request.SourceRevision) {
		return fmt.Errorf("source_revision must be path-safe")
	}
	if request.ExpectedRevision == "" {
		return fmt.Errorf("expected_revision is required")
	}
	if len(request.Files) > 512 || len(request.Skills) > 32 || len(request.History) > 1000 {
		return fmt.Errorf("import contains too many items")
	}
	total := 0
	check := func(files []File) error {
		seen := map[string]bool{}
		for _, file := range files {
			if seen[file.Path] {
				return fmt.Errorf("duplicate file path")
			}
			seen[file.Path] = true
			body, err := Decode(file)
			if err != nil {
				return err
			}
			total += len(body)
			if total > 24*1024*1024 {
				return fmt.Errorf("import exceeds the content limit")
			}
		}
		return nil
	}
	if err := check(request.Files); err != nil {
		return err
	}
	names := map[string]bool{}
	for _, skill := range request.Skills {
		name, err := skilllocal.NormalizeName(skill.Name)
		if err != nil || name != skill.Name || !ValidPath(name) || strings.Contains(name, "/") || strings.HasPrefix(name, ".") || names[name] {
			return fmt.Errorf("invalid or duplicate skill name")
		}
		names[name] = true
		if len(skill.Files) > 128 {
			return fmt.Errorf("skill contains too many files")
		}
		found := false
		for _, file := range skill.Files {
			if file.Path == "SKILL.md" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("custom skill must contain SKILL.md")
		}
		if err = check(skill.Files); err != nil {
			return err
		}
	}
	ids := map[string]bool{}
	for _, message := range request.History {
		if !sourcePattern.MatchString(message.ID) || ids[message.ID] {
			return fmt.Errorf("history requires unique source message IDs")
		}
		ids[message.ID] = true
		switch message.Role {
		case "user", "assistant", "system", "tool":
		default:
			return fmt.Errorf("unsupported history role")
		}
		if len(message.Text) > 256*1024 {
			return fmt.Errorf("history message exceeds the text limit")
		}
	}
	return nil
}

// WriteOnce accepts identical retry content and never overwrites another file.
func WriteOnce(rootPath, relative string, body []byte) error {
	if !ValidPath(relative) {
		return fmt.Errorf("unsafe destination path")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	parts := strings.Split(relative, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return ErrConflict
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if existing, err := filebrowse.OpenFile(rootPath, relative); err == nil {
		current, readErr := io.ReadAll(io.LimitReader(existing, int64(len(body))+1))
		existing.Close()
		if readErr != nil {
			return readErr
		}
		if bytes.Equal(current, body) {
			return nil
		}
		return ErrConflict
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = root.MkdirAll(path.Dir(relative), 0700); err != nil {
		return err
	}
	temp := path.Join(path.Dir(relative), ".import-"+ID(relative, time.Now().UTC().String()))
	file, err := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	_, err = file.Write(body)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// The caller holds the import/write lock. A new destination is checked again
	// before the atomic rename so retry never knowingly replaces user content.
	if _, err = root.Lstat(relative); err == nil {
		return ErrConflict
	} else if !os.IsNotExist(err) {
		return err
	}
	return root.Rename(temp, relative)
}
func InstallSkill(rootPath string, skill Skill) error {
	if err := os.MkdirAll(rootPath, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	if info, err := root.Lstat(skill.Name); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrConflict
		}
		expected := map[string]File{}
		for _, file := range skill.Files {
			expected[file.Path] = file
		}
		err = fs.WalkDir(root.FS(), skill.Name, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel := strings.TrimPrefix(name, skill.Name+"/")
			file, ok := expected[rel]
			if file.Executable && runtime.GOOS != "windows" {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if info.Mode().Perm()&0111 == 0 {
					return ErrConflict
				}
			}
			if !ok || entry.Type()&os.ModeSymlink != 0 {
				return ErrConflict
			}
			existing, err := filebrowse.OpenFile(rootPath, name)
			if err != nil {
				return err
			}
			hash := sha256.New()
			count, err := io.Copy(hash, io.LimitReader(existing, 16*1024*1024+1))
			existing.Close()
			if err != nil {
				return err
			}
			if count > 16*1024*1024 || hex.EncodeToString(hash.Sum(nil)) != file.SHA256 {
				return ErrConflict
			}
			delete(expected, rel)
			return nil
		})
		if err != nil {
			return err
		}
		if len(expected) > 0 {
			return ErrConflict
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	stage := ".legacy-stage-" + ID(skill.Name, time.Now().UTC().String())
	if err = root.Mkdir(stage, 0700); err != nil {
		return err
	}
	defer root.RemoveAll(stage)
	stagePath := filepath.Join(rootPath, stage)
	for _, file := range skill.Files {
		body, err := Decode(file)
		if err != nil {
			return err
		}
		if err = WriteOnce(stagePath, file.Path, body); err != nil {
			return err
		}
		if file.Executable {
			stageRoot, err := root.OpenRoot(stage)
			if err != nil {
				return err
			}
			err = stageRoot.Chmod(file.Path, 0700)
			stageRoot.Close()
			if err != nil {
				return err
			}
		}
	}
	if _, err = root.Lstat(skill.Name); err == nil {
		return ErrConflict
	} else if !os.IsNotExist(err) {
		return err
	}
	return root.Rename(stage, skill.Name)
}

func RecordID(request Request) string {
	if request.SourceRevision == "" {
		return ID(request.SourceID, request.SourceSession)
	}
	return ID(request.SourceID, request.SourceSession+"\x00"+request.SourceRevision)
}
