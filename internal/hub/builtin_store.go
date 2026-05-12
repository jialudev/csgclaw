package hub

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	builtinTemplatesRoot = "embed/templates"
)

//go:embed embed/templates
var builtinTemplateFS embed.FS

type BuiltinStore struct{}

func NewBuiltinStore() *BuiltinStore {
	return &BuiltinStore{}
}

func (s *BuiltinStore) List(context.Context) ([]Template, error) {
	entries, err := fs.ReadDir(builtinTemplateFS, builtinTemplatesRoot)
	if err != nil {
		return nil, fmt.Errorf("read builtin hub templates: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := strings.TrimSpace(entry.Name())
		if err := validateLocalTemplateID(id); err != nil {
			return nil, fmt.Errorf("invalid builtin hub template %q: %w", id, err)
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)

	items := make([]Template, 0, len(ids))
	for _, id := range ids {
		item, err := s.Get(context.Background(), id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *BuiltinStore) Get(_ context.Context, id string) (Template, error) {
	id, manifest, err := loadManifestFS(builtinTemplateFS, s.manifestPath(id), "builtin")
	if err != nil {
		return Template{}, err
	}
	updatedAt, err := parseManifestUpdatedAt(manifest.UpdatedAt)
	if err != nil {
		return Template{}, fmt.Errorf("validate builtin hub manifest %q: %w", id, err)
	}
	return Template{
		ID:          id,
		Name:        manifest.Name,
		Description: manifest.Description,
		RuntimeKind: manifest.RuntimeKind,
		Image:       manifest.Image,
		WorkspaceRef: WorkspaceRef{
			Kind: WorkspaceKindDir,
			Path: s.workspacePath(id),
		},
		UpdatedAt: updatedAt,
	}, nil
}

func (s *BuiltinStore) FetchWorkspace(_ context.Context, id string) (WorkspaceRef, error) {
	id = strings.TrimSpace(id)
	if err := validateLocalTemplateID(id); err != nil {
		return WorkspaceRef{}, err
	}
	if _, _, err := loadManifestFS(builtinTemplateFS, s.manifestPath(id), "builtin"); err != nil {
		return WorkspaceRef{}, err
	}
	root := s.workspacePath(id)
	tmpDir, err := os.MkdirTemp("", "csgclaw-hub-builtin-*")
	if err != nil {
		return WorkspaceRef{}, fmt.Errorf("create builtin hub workspace temp dir: %w", err)
	}
	if err := copyWorkspaceTreeFS(builtinTemplateFS, root, tmpDir, "builtin hub workspace"); err != nil {
		_ = os.RemoveAll(tmpDir)
		return WorkspaceRef{}, err
	}
	return WorkspaceRef{Kind: WorkspaceKindDir, Path: tmpDir}, nil
}

func (s *BuiltinStore) Publish(context.Context, PublishSpec) (Template, error) {
	return Template{}, ErrRegistryNotWritable
}

func (s *BuiltinStore) manifestPath(id string) string {
	return filepath.ToSlash(filepath.Join(builtinTemplatesRoot, id, localManifestFileName))
}

func (s *BuiltinStore) workspacePath(id string) string {
	return filepath.ToSlash(filepath.Join(builtinTemplatesRoot, id, localWorkspaceDirName))
}
