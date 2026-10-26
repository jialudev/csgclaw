package contract

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestPersistentFilesRetainIdentityAndAgentScope(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := NewFileStore()
	if err := s.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	f, err := s.Scope("a").Create(ctx, FileCreateRequest{Name: "a.txt", MIMEType: "text/plain", SizeBytes: 5}, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	reloaded := NewFileStore()
	if err = reloaded.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	content, err := reloaded.Scope("a").Get(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(content.Content)
	content.Content.Close()
	if string(body) != "hello" || content.Metadata != f.OutputFileMetadata {
		t.Fatal("content or ID changed")
	}
	if _, err = reloaded.Scope("b").Get(ctx, f.ID); ErrorCodeOf(err) != ErrorFileNotFound {
		t.Fatalf("cross-agent: %v", err)
	}
	if err = reloaded.Scope("a").Delete(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	again := NewFileStore()
	if err = again.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	if _, err = again.Scope("a").Get(ctx, f.ID); ErrorCodeOf(err) != ErrorFileNotFound {
		t.Fatal("deleted file reappeared")
	}
}

func TestFileImportIdempotencySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := NewFileStore()
	if err := s.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	request := FileCreateRequest{IdempotencyKey: "source:session:path", Name: "a.txt", MIMEType: "text/plain", SizeBytes: 5}
	first, err := s.Scope("a").Create(ctx, request, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	reloaded := NewFileStore()
	if err = reloaded.EnablePersistence(dir); err != nil {
		t.Fatal(err)
	}
	second, err := reloaded.Scope("a").Create(ctx, request, strings.NewReader("hello"))
	if err != nil || first.ID != second.ID {
		t.Fatalf("duplicate: %+v %v", second, err)
	}
	if _, err = reloaded.Scope("a").Create(ctx, request, strings.NewReader("other")); err == nil {
		t.Fatal("idempotency key changed content")
	}
}

func TestActiveInputLeaseBlocksDeletionButDownloadDoesNot(t *testing.T) {
	ctx := context.Background()
	s := NewFileStore()
	files := s.Scope("a")
	created, err := files.Create(ctx, FileCreateRequest{Name: "a.txt", MIMEType: "text/plain", SizeBytes: 5}, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	_, release, turnErr := s.ResolveInput("a", created.ID)
	if turnErr != nil {
		t.Fatal(turnErr)
	}
	if err = files.Delete(ctx, created.ID); ErrorCodeOf(err) != ErrorConversationBusy {
		t.Fatalf("active input deleted: %v", err)
	}
	release()
	release()
	if err = files.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
}
