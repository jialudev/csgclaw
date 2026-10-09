package im

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSaveLargeMetadataSpillsToBlob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions", "room.jsonl")
	message := Message{
		ID: "turn-final", SenderID: "agent", Content: "Short answer", CreatedAt: time.Now().UTC(),
		Metadata: map[string]any{"csgclaw": map[string]any{
			"delivery_kind": "final",
			"turn_progress": map[string]any{"items": []any{
				map[string]any{"kind": "tool", "output": strings.Repeat("工具输出\n", 12000)},
			}},
		}},
	}
	if err := saveMessagesJSONL(path, "room", []Message{message}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.TrimSpace(data)) > maxSessionJSONLLineBytes {
		t.Fatalf("session line exceeds budget: %d bytes", len(data))
	}
	loaded, err := loadMessagesJSONL(path, "room")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Content != message.Content || !reflect.DeepEqual(loaded[0].Metadata, message.Metadata) {
		t.Fatal("large message metadata did not survive save and reload")
	}
}

func TestLoadLegacyBlobKeepsInlineMetadata(t *testing.T) {
	dir := t.TempDir()
	metadata := map[string]any{"csgclaw": map[string]any{"delivery_kind": "final"}}
	line := messageToSessionLine(Message{ID: "legacy", SenderID: "agent", CreatedAt: time.Now().UTC(), Metadata: metadata})
	line.BlobRef = "blobs/room/legacy.json"
	path := filepath.Join(dir, filepath.FromSlash(line.BlobRef))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"content":"Legacy response"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	message, err := decodeSessionMessageLine(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	if message.Content != "Legacy response" || !reflect.DeepEqual(message.Metadata, metadata) {
		t.Fatal("legacy blob lost its inline metadata")
	}
}

func TestLegacyOversizedMetadataMigratesOnNextSave(t *testing.T) {
	for _, format := range []string{"inline", "legacy_blob"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			state := Bootstrap{
				CurrentUserID: AdminUserID,
				Users:         []User{{ID: AdminUserID, Name: "Admin"}, {ID: ManagerUserID, Name: "Manager"}},
				Rooms: []Room{
					{ID: "old-room", Title: "Historical room", Members: []string{AdminUserID, ManagerUserID}},
					{ID: "other-room", Title: "Other room", Members: []string{AdminUserID, ManagerUserID}},
				},
			}
			if err := SaveBootstrap(path, state); err != nil {
				t.Fatal(err)
			}
			original := Message{
				ID: "historical-final", SenderID: ManagerUserID, Content: "Historical answer", CreatedAt: time.Now().UTC(),
				Metadata: map[string]any{"csgclaw": map[string]any{
					"delivery_kind": "final",
					"turn_progress": map[string]any{"status": "succeeded", "items": []any{
						map[string]any{"kind": "reasoning", "text": strings.Repeat("R", 70*1024)},
					}},
				}},
			}
			// Write an old-format row directly: the new writer would already spill it.
			row := messageToSessionLine(original)
			if format == "legacy_blob" {
				row.Content = ""
				row.BlobRef = "blobs/old-room/historical-final.json"
				blobPath := filepath.Join(filepath.Dir(path), "sessions", filepath.FromSlash(row.BlobRef))
				if err := os.MkdirAll(filepath.Dir(blobPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(blobPath, []byte(`{"content":"Historical answer"}`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			data, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) <= maxSessionJSONLLineBytes {
				t.Fatal("fixture must exceed the old write limit")
			}
			sessionPath := filepath.Join(filepath.Dir(path), "sessions", "old-room.jsonl")
			if err := os.WriteFile(sessionPath, append(data, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			service, err := NewServiceFromPath(path)
			if err != nil {
				t.Fatalf("load historical oversized metadata: %v", err)
			}
			unchanged, err := os.ReadFile(sessionPath)
			if err != nil || !bytes.Equal(unchanged, append(data, '\n')) {
				t.Fatalf("startup must leave legacy history unchanged: %v", err)
			}
			if _, err := service.CreateMessage(CreateMessageRequest{RoomID: "other-room", SenderID: AdminUserID, Content: "Hello"}); err != nil {
				t.Fatalf("historical message blocks another room: %v", err)
			}
			if _, err := service.CreateRoom(CreateRoomRequest{Type: "on_demand", Title: "New room", CreatorID: AdminUserID, ManagerID: ManagerUserID}); err != nil {
				t.Fatalf("historical message blocks room creation: %v", err)
			}
			data, err = os.ReadFile(sessionPath)
			if err != nil || len(bytes.TrimSpace(data)) > maxSessionJSONLLineBytes {
				t.Fatalf("historical record was not migrated: bytes=%d, error=%v", len(data), err)
			}
			loaded, err := loadMessagesJSONL(sessionPath, "old-room")
			if err != nil || len(loaded) != 1 || !reflect.DeepEqual(loaded[0], original) {
				t.Fatalf("migration changed historical message content or metadata: %v", err)
			}
		})
	}
}

func assertSessionFileUnchanged(t *testing.T, path string, before os.FileInfo) {
	t.Helper()
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("unchanged file was rewritten: %s", path)
	}
}

func TestSessionSaveReusesUnchangedFilesAndRepairsMissingBlob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions", "room.jsonl")
	messages := []Message{{ID: "large", SenderID: "agent", Content: strings.Repeat("A", 70*1024), CreatedAt: time.Now().UTC()}}
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	blobDir := filepath.Join(filepath.Dir(path), sessionBlobsDirName, "room")
	entries, err := os.ReadDir(blobDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one blob: %v", err)
	}
	blobPath := filepath.Join(blobDir, entries[0].Name())
	sessionInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	blobInfo, err := os.Stat(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	assertSessionFileUnchanged(t, path, sessionInfo)
	assertSessionFileUnchanged(t, blobPath, blobInfo)
	if err := os.Remove(blobPath); err != nil {
		t.Fatal(err)
	}
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	assertSessionFileUnchanged(t, path, sessionInfo)
	loaded, err := loadMessagesJSONL(path, "room")
	if err != nil || !reflect.DeepEqual(loaded, messages) {
		t.Fatalf("missing blob was not restored: %v", err)
	}
}

func TestSessionSaveDetectsSameSizeChangeAfterMatchingPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "room.jsonl")
	messages := []Message{
		{ID: "prefix", SenderID: "agent", Content: strings.Repeat("A", 35*1024), CreatedAt: time.Now().UTC()},
		{ID: "reply", SenderID: "agent", Content: "old reply", CreatedAt: time.Now().UTC()},
	}
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	messages[1].Content = "new reply"
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadMessagesJSONL(path, "room")
	if err != nil || !reflect.DeepEqual(loaded, messages) {
		t.Fatalf("same-size update was not saved: %v", err)
	}
}

func TestSessionSaveAppendsOnlyNewMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "room.jsonl")
	messages := []Message{{ID: "first", SenderID: "agent", Content: strings.Repeat("A", 35*1024), CreatedAt: time.Now().UTC()}}
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	messages = append(messages,
		Message{ID: "second", SenderID: "agent", Content: "New reply", CreatedAt: time.Now().UTC()},
		Message{ID: "third", SenderID: "agent", Content: strings.Repeat("B", 70*1024), CreatedAt: time.Now().UTC()},
	)
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || after.Size() <= before.Size() {
		t.Fatalf("new messages should append without replacing the existing file: %v", err)
	}
	loaded, err := loadMessagesJSONL(path, "room")
	if err != nil || !reflect.DeepEqual(loaded, messages) {
		t.Fatalf("appended messages did not survive reload: %v", err)
	}
	// Updating an existing message must replace its row, even when the new
	// snapshot is longer. Do not turn progress updates into duplicate messages.
	messages[0].Content += "updated"
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadMessagesJSONL(path, "room")
	if err != nil || !reflect.DeepEqual(loaded, messages) {
		t.Fatalf("updating an old message duplicated or lost records: %v", err)
	}
	// Removing a message must not leave a stale suffix on disk.
	messages = messages[:1]
	if err := saveMessagesJSONL(path, "room", messages); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadMessagesJSONL(path, "room")
	if err != nil || !reflect.DeepEqual(loaded, messages) {
		t.Fatalf("removed messages remain on disk: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(filepath.Dir(path), sessionBlobsDirName, "room"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("removed message blob remains: %v", err)
	}
}

func TestSessionLoadHandlesInterruptedAppendWithoutStartupWrites(t *testing.T) {
	message := Message{ID: "committed", SenderID: "agent", Content: "Keep this", CreatedAt: time.Now().UTC()}
	line, err := json.Marshal(messageToSessionLine(message))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, tail string
		first      bool
		wantError  bool
	}{
		{name: "partial_object", tail: `{"id":"next","content":"half`},
		{name: "partial_array", tail: `{"mentions":[{"id":`},
		{name: "partial_first_message", tail: `{"id":`, first: true},
		{name: "complete_legacy_row_without_newline", tail: string(line), first: true},
		{name: "malformed_tail", tail: `{"id":!}`, wantError: true},
		{name: "malformed_complete_line", tail: "{\"id\":\n", wantError: true},
		{name: "malformed_middle_line", tail: "{\"id\":\n" + string(line) + "\n", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "room.jsonl")
			data := []byte(tc.tail)
			var expected []Message
			if !tc.first {
				data = append(append(bytes.Clone(line), '\n'), data...)
				expected = []Message{message}
			} else if tc.tail == string(line) {
				expected = []Message{message}
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			loaded, err := loadMessagesJSONL(path, "room")
			if tc.wantError {
				if err == nil {
					t.Fatal("corrupt complete records must not be silently discarded")
				}
				return
			}
			if err != nil || len(loaded) != len(expected) || (len(expected) > 0 && !reflect.DeepEqual(loaded, expected)) {
				t.Fatalf("unexpected recovered history: %#v, error: %v", loaded, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, data) {
				t.Fatalf("reading history modified the original file: %v", err)
			}
			loaded = append(loaded, Message{ID: "retry", SenderID: "agent", Content: "Retry", CreatedAt: time.Now().UTC()})
			if err := saveMessagesJSONL(path, "room", loaded); err != nil {
				t.Fatal(err)
			}
			reloaded, err := loadMessagesJSONL(path, "room")
			if err != nil || !reflect.DeepEqual(reloaded, loaded) {
				t.Fatalf("normal save did not replace the incomplete tail: %v", err)
			}
		})
	}
}

type failingSessionAppend struct {
	*os.File
	writeErr, syncErr, rollbackErr error
	shortWrite                     bool
}

func (f *failingSessionAppend) Write(data []byte) (int, error) {
	if f.shortWrite || f.writeErr != nil {
		n, err := f.File.Write(data[:len(data)/2])
		return n, errors.Join(err, f.writeErr)
	}
	return f.File.Write(data)
}

func (f *failingSessionAppend) Sync() error {
	if err := f.syncErr; err != nil {
		f.syncErr = nil // Fail the write sync, allow rollback to sync.
		return err
	}
	return f.File.Sync()
}

func (f *failingSessionAppend) Truncate(size int64) error {
	if f.rollbackErr != nil {
		return f.rollbackErr
	}
	return f.File.Truncate(size)
}

func TestSessionAppendRollsBackWriteFailures(t *testing.T) {
	writeFailure := errors.New("disk write failed")
	syncFailure := errors.New("disk sync failed")
	rollbackFailure := errors.New("truncate failed")
	for _, tc := range []struct {
		name string
		file failingSessionAppend
		want error
	}{
		{"partial_write", failingSessionAppend{writeErr: writeFailure}, writeFailure},
		{"short_write", failingSessionAppend{shortWrite: true}, io.ErrShortWrite},
		{"sync", failingSessionAppend{syncErr: syncFailure}, syncFailure},
		{"rollback", failingSessionAppend{writeErr: writeFailure, rollbackErr: rollbackFailure}, rollbackFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "room.jsonl")
			original := []byte("{\"id\":\"committed\"}\n")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.Seek(0, io.SeekEnd); err != nil {
				t.Fatal(err)
			}
			tc.file.File = file
			err = writeSessionAppend(&tc.file, []byte("{\"id\":\"next\"}\n"), int64(len(original)))
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.HasPrefix(after, original) {
				t.Fatalf("append damaged committed bytes: %v", err)
			}
			if tc.file.rollbackErr == nil && !bytes.Equal(after, original) {
				t.Fatal("failed append was not rolled back")
			}
		})
	}
}

func TestSessionAppendRejectsChangedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "room.jsonl")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(path, []byte("replacement\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appendSessionJSONL(path, []byte("next\n"), previous); err == nil {
		t.Fatal("must reject a file replaced after comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "replacement\n" {
		t.Fatalf("changed file was overwritten: %v", err)
	}
}

func TestFailedSessionSavePreservesCommittedMessagesAndBlobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions", "room.jsonl")
	original := []Message{
		{ID: "large", SenderID: "agent", Content: strings.Repeat("A", 70*1024), CreatedAt: time.Now().UTC()},
		{ID: "later", SenderID: "agent", Content: "Later message", CreatedAt: time.Now().UTC()},
	}
	if err := saveMessagesJSONL(path, "room", original); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := append([]Message(nil), original...)
	updated[0].Content = strings.Repeat("B", 70*1024)
	updated[1].Metadata = map[string]any{"invalid": make(chan struct{})}
	if err := saveMessagesJSONL(path, "room", updated); err == nil {
		t.Fatal("expected message encoding failure")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed save changed the committed session: %v", err)
	}
	loaded, err := loadMessagesJSONL(path, "room")
	if err != nil || !reflect.DeepEqual(loaded, original) {
		t.Fatalf("failed save changed committed messages or blobs: %v", err)
	}
	// A later successful write also collects blobs left by the failed attempt.
	if err := saveMessagesJSONL(path, "room", original); err != nil {
		t.Fatal(err)
	}
	blobs, err := os.ReadDir(filepath.Join(filepath.Dir(path), sessionBlobsDirName, "room"))
	if err != nil || len(blobs) != 1 {
		t.Fatalf("unreferenced blobs remain: count=%d, error=%v", len(blobs), err)
	}
}

func TestSaveLargeMessageSpillsToBlob(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	roomID := "room-large"
	large := strings.Repeat("Z", 70*1024)

	state := Bootstrap{
		CurrentUserID: "u-admin",
		Users:         []User{{ID: "u-admin", Name: "admin"}},
		Rooms: []Room{{
			ID:      roomID,
			Title:   "large",
			Members: []string{"u-admin"},
			Messages: []Message{{
				ID:        "msg-large-1",
				SenderID:  "u-admin",
				Kind:      MessageKindMessage,
				Content:   large,
				CreatedAt: time.Date(2026, 5, 25, 3, 0, 0, 0, time.UTC),
			}},
		}},
	}
	if err := SaveBootstrap(statePath, state); err != nil {
		t.Fatalf("SaveBootstrap() error = %v", err)
	}

	sessionPath := filepath.Join(dir, "sessions", roomID+".jsonl")
	sessionData, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("ReadFile(session) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(sessionData)), "\n")
	if len(lines) != 1 {
		t.Fatalf("session lines = %d, want 1", len(lines))
	}
	if len(lines[0]) > maxSessionJSONLLineBytes {
		t.Fatalf("jsonl line bytes = %d, want <= %d", len(lines[0]), maxSessionJSONLLineBytes)
	}

	var record sessionMessageLine
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("Unmarshal(session line) error = %v", err)
	}
	if record.BlobRef == "" {
		t.Fatal("blob_ref is empty, want spillover reference")
	}
	if record.Content != "" {
		t.Fatalf("inline content = %d bytes, want empty with blob_ref", len(record.Content))
	}

	blobPath := filepath.Join(dir, "sessions", filepath.FromSlash(record.BlobRef))
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("ReadFile(blob) error = %v", err)
	}
	var blob sessionMessageBlob
	if err := json.Unmarshal(blobData, &blob); err != nil {
		t.Fatalf("Unmarshal(blob) error = %v", err)
	}
	if blob.Content != large {
		t.Fatalf("blob content len = %d, want %d", len(blob.Content), len(large))
	}

	loaded, err := LoadBootstrap(statePath)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	if len(loaded.Rooms) != 1 || len(loaded.Rooms[0].Messages) != 1 {
		t.Fatalf("loaded rooms = %+v, want one large message", loaded.Rooms)
	}
	if loaded.Rooms[0].Messages[0].Content != large {
		t.Fatalf("loaded content len = %d, want %d", len(loaded.Rooms[0].Messages[0].Content), len(large))
	}
}

func TestSaveMessageKeepsSlashInvocationAsContentOnly(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	roomID := "room-skill"

	state := Bootstrap{
		CurrentUserID: "u-admin",
		Users:         []User{{ID: "u-admin", Name: "admin"}},
		Rooms: []Room{{
			ID:      roomID,
			Title:   "skill",
			Members: []string{"u-admin"},
			Messages: []Message{{
				ID:        "msg-skill-1",
				SenderID:  "u-admin",
				Kind:      MessageKindMessage,
				Content:   "/custom do it",
				Metadata:  map[string]any{"openclaw": map[string]any{"delivery_kind": "final"}},
				CreatedAt: time.Date(2026, 5, 25, 3, 30, 0, 0, time.UTC),
			}},
		}},
	}
	if err := SaveBootstrap(statePath, state); err != nil {
		t.Fatalf("SaveBootstrap() error = %v", err)
	}

	sessionPath := filepath.Join(dir, "sessions", roomID+".jsonl")
	sessionData, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("ReadFile(session) error = %v", err)
	}
	if !strings.Contains(string(sessionData), `"content":"/custom do it"`) {
		t.Fatalf("session = %q, want original slash invocation content", string(sessionData))
	}
	if !strings.Contains(string(sessionData), `"metadata":{"openclaw":{"delivery_kind":"final"}}`) {
		t.Fatalf("session = %q, want message metadata persisted", string(sessionData))
	}
	if strings.Contains(string(sessionData), "agent_content") || strings.Contains(string(sessionData), "Follow custom rules") {
		t.Fatalf("session = %q, want no hidden skill payload", string(sessionData))
	}

	loaded, err := LoadBootstrap(statePath)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	if len(loaded.Rooms) != 1 || len(loaded.Rooms[0].Messages) != 1 {
		t.Fatalf("loaded rooms = %+v, want one slash invocation message", loaded.Rooms)
	}
	if loaded.Rooms[0].Messages[0].Content != "/custom do it" {
		t.Fatalf("loaded content = %q, want original slash invocation", loaded.Rooms[0].Messages[0].Content)
	}
	openclaw, ok := loaded.Rooms[0].Messages[0].Metadata["openclaw"].(map[string]any)
	if !ok || openclaw["delivery_kind"] != "final" {
		t.Fatalf("loaded metadata = %#v, want openclaw final", loaded.Rooms[0].Messages[0].Metadata)
	}
}

func TestSaveEmptyMessagesTruncatesSessionAndRemovesBlobs(t *testing.T) {
	dir := t.TempDir()
	roomID := "room-clear"
	path := filepath.Join(dir, "sessions", roomID+".jsonl")
	large := strings.Repeat("B", 70*1024)

	if err := saveMessagesJSONL(path, roomID, []Message{{
		ID:        "msg-large",
		SenderID:  "u-admin",
		Content:   large,
		CreatedAt: time.Date(2026, 5, 25, 5, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatalf("saveMessagesJSONL(large) error = %v", err)
	}
	blobDir := filepath.Join(dir, "sessions", sessionBlobsDirName, roomID)
	if _, err := os.Stat(blobDir); err != nil {
		t.Fatalf("Stat(blob dir) error = %v", err)
	}

	if err := saveMessagesJSONL(path, roomID, nil); err != nil {
		t.Fatalf("saveMessagesJSONL(empty) error = %v", err)
	}
	sessionData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(session) error = %v", err)
	}
	if len(sessionData) != 0 {
		t.Fatalf("session bytes = %d, want 0", len(sessionData))
	}
	if _, err := os.Stat(blobDir); !os.IsNotExist(err) {
		t.Fatalf("Stat(blob dir) error = %v, want not exist", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveMessagesJSONL(path, roomID, nil); err != nil {
		t.Fatal(err)
	}
	assertSessionFileUnchanged(t, path, before)
}

func TestLoadLegacyOversizedInlineLine(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	roomID := "room-legacy"
	large := strings.Repeat("L", 70*1024)

	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	legacy := Message{
		ID:        "msg-legacy-fat",
		SenderID:  "u-admin",
		Kind:      MessageKindMessage,
		Content:   large,
		CreatedAt: time.Date(2026, 5, 25, 4, 0, 0, 0, time.UTC),
	}
	line, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("Marshal(legacy) error = %v", err)
	}
	if len(line) <= maxSessionJSONLLineBytes {
		t.Fatalf("legacy line bytes = %d, want > %d for repro", len(line), maxSessionJSONLLineBytes)
	}
	sessionPath := filepath.Join(dir, "sessions", roomID+".jsonl")
	if err := os.WriteFile(sessionPath, append(line, '\n'), 0o600); err != nil {
		t.Fatalf("WriteFile(session) error = %v", err)
	}

	stateJSON := `{
  "current_user_id": "u-admin",
  "users": [{"id": "u-admin", "name": "admin"}],
  "rooms": [{
    "id": "` + roomID + `",
    "title": "legacy",
    "members": ["u-admin"],
    "messages": "sessions/` + roomID + `.jsonl"
  }]
}`
	if err := os.WriteFile(statePath, []byte(stateJSON), 0o600); err != nil {
		t.Fatalf("WriteFile(state.json) error = %v", err)
	}

	loaded, err := LoadBootstrap(statePath)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	if len(loaded.Rooms[0].Messages) != 1 || loaded.Rooms[0].Messages[0].Content != large {
		t.Fatalf("loaded message = %+v, want legacy fat content", loaded.Rooms[0].Messages[0])
	}

	if err := SaveBootstrap(statePath, loaded); err != nil {
		t.Fatalf("SaveBootstrap(migrate) error = %v", err)
	}
	sessionData, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("ReadFile(session after migrate) error = %v", err)
	}
	migratedLine := strings.TrimSpace(strings.Split(string(sessionData), "\n")[0])
	if len(migratedLine) > maxSessionJSONLLineBytes {
		t.Fatalf("migrated jsonl line bytes = %d, want <= %d", len(migratedLine), maxSessionJSONLLineBytes)
	}
	var record sessionMessageLine
	if err := json.Unmarshal([]byte(migratedLine), &record); err != nil {
		t.Fatalf("Unmarshal(migrated line) error = %v", err)
	}
	if record.BlobRef == "" {
		t.Fatal("migrated line missing blob_ref")
	}
}
