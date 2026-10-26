package legacyimport

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testFile(path, content string) File {
	sum := sha256.Sum256([]byte(content))
	return File{Path: path, MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte(content)), SHA256: hex.EncodeToString(sum[:])}
}
func TestImportValidatesHashAndRejectsUnsafePaths(t *testing.T) {
	req := Request{SourceID: "csgbot", SourceSession: "s1", ExpectedRevision: "r", Files: []File{testFile("reports/a.txt", "content")}}
	if err := Validate(req); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside", "/absolute", "a/../../outside", `a\b`, "a/./b"} {
		req.Files[0].Path = name
		if Validate(req) == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	req.Files[0] = testFile("a.txt", "content")
	req.Files[0].SHA256 = "wrong"
	if Validate(req) == nil {
		t.Fatal("accepted incorrect hash")
	}
}
func TestWriteOnceNeverOverwritesOrFollowsLinks(t *testing.T) {
	root := t.TempDir()
	if err := WriteOnce(root, "imports/a.txt", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := WriteOnce(root, "imports/a.txt", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := WriteOnce(root, "imports/a.txt", []byte("new")); !errors.Is(err, ErrConflict) {
		t.Fatal("overwrote existing file", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip(err)
	}
	if err := WriteOnce(root, "link/secret", []byte("new")); !errors.Is(err, ErrConflict) {
		t.Fatal("followed link", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "secret")); !os.IsNotExist(err) {
		t.Fatal("wrote outside root")
	}
}
func TestSkillStageIsAtomicAndRepeatable(t *testing.T) {
	root := t.TempDir()
	skill := Skill{Name: "legacy-skill", Files: []File{testFile("SKILL.md", "# Legacy"), testFile("scripts/helper.py", "print('hello')")}}
	skill.Files[1].Executable = true
	if err := InstallSkill(root, skill); err != nil {
		t.Fatal(err)
	}
	if err := InstallSkill(root, skill); err != nil {
		t.Fatal("identical retry failed", err)
	}
	skill.Files[0] = testFile("SKILL.md", "# Changed")
	if err := InstallSkill(root, skill); !errors.Is(err, ErrConflict) {
		t.Fatal("changed skill overwrote old content", err)
	}
	old, _ := os.ReadFile(filepath.Join(root, "legacy-skill", "SKILL.md"))
	if string(old) != "# Legacy" {
		t.Fatal("changed published skill")
	}
}
