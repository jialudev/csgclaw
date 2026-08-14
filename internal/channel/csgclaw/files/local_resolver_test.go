package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"csgclaw/internal/channel"
	"csgclaw/internal/channelbridge"
	"csgclaw/internal/im"
)

type staticWorkspaceResolver struct {
	root string
}

func (r staticWorkspaceResolver) WorkspaceRootByID(string) (string, error) {
	return r.root, nil
}

func TestLocalResolverReturnsAbsoluteManagedSourcePath(t *testing.T) {
	service, err := im.NewServiceFromPath(filepath.Join(t.TempDir(), "im", "state.json"))
	if err != nil {
		t.Fatalf("NewServiceFromPath() error = %v", err)
	}
	if _, _, err := service.EnsureAgentUser(im.EnsureAgentUserRequest{ID: "agent-worker", Name: "worker", Role: "worker"}); err != nil {
		t.Fatalf("EnsureAgentUser() error = %v", err)
	}
	room, err := service.CreateRoom(im.CreateRoomRequest{
		Title: "Ops", CreatorID: "user-admin", MemberIDs: []string{"user-worker"},
	})
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	payload := []byte("image fixture")
	message, err := service.CreateMessage(im.CreateMessageRequest{
		RoomID: room.ID, SenderID: "user-admin", Attachments: []im.MessageAttachmentUpload{{
			Name: "diagram.png", MediaType: "image/png", Data: payload,
		}},
	})
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}
	if len(message.Attachments) != 1 {
		t.Fatalf("attachments = %+v, want one attachment", message.Attachments)
	}

	workspaceRoot := t.TempDir()
	resolver, err := NewLocalResolver(service, staticWorkspaceResolver{root: workspaceRoot})
	if err != nil {
		t.Fatalf("NewLocalResolver() error = %v", err)
	}
	attachment := message.Attachments[0]
	input, release, err := resolver.Resolve(
		context.Background(),
		channel.Binding{AgentID: "agent-worker"},
		channelbridge.BotEvent{RoomID: room.ID, MessageID: message.ID, Attachments: message.Attachments},
		attachment,
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !filepath.IsAbs(input.SourcePath) {
		t.Fatalf("SourcePath = %q, want absolute path", input.SourcePath)
	}
	if got, err := os.ReadFile(input.SourcePath); err != nil || string(got) != string(payload) {
		t.Fatalf("resolved source = %q, err=%v, want %q", string(got), err, string(payload))
	}
	if release == nil {
		t.Fatal("release = nil, want managed attachment cleanup")
	}
	release()
	if _, err := os.Stat(input.SourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists after release: %v", err)
	}
}

func TestResolveManagedAttachmentSourcePathRejectsOutsideWorkspace(t *testing.T) {
	workspaceRoot := t.TempDir()
	managedRoot := filepath.Join(workspaceRoot, ".csgclaw", "attachments")
	if err := os.MkdirAll(managedRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveManagedAttachmentSourcePath(workspaceRoot, outsidePath); err == nil {
		t.Fatal("resolveManagedAttachmentSourcePath() error = nil, want outside path rejected")
	}
}
