package contract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"csgclaw/internal/controlplane"
)

type durableFile struct {
	AgentID         string             `json:"agent_id"`
	Metadata        OutputFileMetadata `json:"metadata"`
	ConversationKey ConversationKey    `json:"conversation_key,omitempty"`
	TurnID          TurnID             `json:"turn_id,omitempty"`
}
type diskFiles struct {
	root  *os.Root
	index *controlplane.Store
}

func diskFileName(agent, id string) string {
	sum := sha256.Sum256([]byte(agent + "\x00" + id))
	return hex.EncodeToString(sum[:]) + ".bin"
}

// EnablePersistence is a startup-only operation; content is verified while
// rebuilding immutable snapshots. Invalid records fail startup rather than
// returning successful downloads with missing or changed contents.
func (s *FileStore) EnablePersistence(dir string) (retErr error) {
	if s.disk != nil {
		return fmt.Errorf("file persistence is already enabled")
	}
	index, err := controlplane.Open(filepath.Join(dir, "index"))
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		index.Close()
		return err
	}
	disk := &diskFiles{root: root, index: index}
	defer func() {
		if retErr != nil {
			root.Close()
			index.Close()
		}
	}()
	raws, err := index.Records()
	if err != nil {
		return err
	}
	for _, raw := range raws {
		var r durableFile
		if json.Unmarshal(raw, &r) != nil || r.AgentID == "" {
			return fmt.Errorf("invalid file metadata record")
		}
		if err := validateFileIdentity(r.Metadata.ID); err != nil {
			return err
		}
		f, err := root.Open(diskFileName(r.AgentID, r.Metadata.ID))
		if err != nil {
			return err
		}
		snapshot, err := NewOutputFile(context.Background(), r.Metadata, f)
		f.Close()
		if err != nil {
			return err
		}
		snapshot.ID = r.Metadata.ID
		if err := s.put(r.AgentID, storedFile{file: snapshot, conversationKey: r.ConversationKey, turnID: r.TurnID}); err != nil {
			snapshot.Cleanup()
			return err
		}
	}
	s.disk = disk
	return nil
}
func (d *diskFiles) put(agent string, file storedFile) error {
	src, err := file.file.Open(context.Background())
	if err != nil {
		return err
	}
	defer src.Close()
	name := diskFileName(agent, file.file.ID)
	f, err := d.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		// An acknowledged record would have been restored into the index already.
		// This is an unindexed blob left between content and index publication.
		if err = d.root.Remove(name); err != nil {
			return err
		}
		f, err = d.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(f, src)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		d.root.Remove(name)
		return err
	}
	record := durableFile{AgentID: agent, Metadata: file.file.OutputFileMetadata, ConversationKey: file.conversationKey, TurnID: file.turnID}
	if err = d.index.Write(agent+"\x00"+file.file.ID, record); err != nil {
		d.root.Remove(name)
		return err
	}
	return nil
}
func (d *diskFiles) remove(agent, id string) error {
	// Remove the index first: a crash may leave an unreferenced blob, never an
	// acknowledged file whose metadata points at deleted content.
	if err := d.index.Remove(agent + "\x00" + id); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := d.root.Remove(diskFileName(agent, id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
func (s *FileStore) Persistent() bool { return s != nil && s.disk != nil }
func validateFileIdentity(id string) error {
	if !strings.HasPrefix(id, "file-") || len(id) != 37 {
		return fmt.Errorf("invalid file ID")
	}
	_, err := hex.DecodeString(strings.TrimPrefix(id, "file-"))
	return err
}

// ClosePersistence releases directory handles after all executions have stopped.
func (s *FileStore) ClosePersistence() error {
	if s == nil || s.disk == nil {
		return nil
	}
	rootErr := s.disk.root.Close()
	indexErr := s.disk.index.Close()
	if rootErr != nil {
		return rootErr
	}
	return indexErr
}
