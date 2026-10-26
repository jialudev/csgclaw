// Package controlplane stores durable control API records independently from
// Runtime adapters. Callers serialize mutations; every replacement is atomic.
package controlplane

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	root *os.Root
	ID   string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{root: root}
	err = s.Read("backend", &s.ID)
	if os.IsNotExist(err) {
		b := make([]byte, 16)
		if _, err = rand.Read(b); err == nil {
			s.ID = hex.EncodeToString(b)
			err = s.Write("backend", s.ID)
		}
	}
	if err != nil {
		root.Close()
		return nil, err
	}
	if s.ID == "" {
		root.Close()
		return nil, fmt.Errorf("backend identity is empty")
	}
	return s, nil
}
func keyName(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:]) + ".json"
}
func (s *Store) Read(key string, out any) error {
	f, err := s.root.Open(keyName(key))
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(out)
}
func (s *Store) Write(key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.WriteBytes(keyName(key), b)
}
func (s *Store) WriteBytes(name string, data []byte) error {
	// Names are internal hashes, never user paths.
	if filepath.Base(name) != name {
		return fmt.Errorf("invalid record name")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	tmp := ".tmp-" + hex.EncodeToString(b)
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return s.root.Rename(tmp, name)
}

// Walk visits one record at a time so startup and scans do not retain all
// histories or import payloads in memory at once.
func (s *Store) Walk(visit func(json.RawMessage) error) error {
	f, err := s.root.Open(".")
	if err != nil {
		return err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return err
	}
	for _, name := range names {
		if filepath.Ext(name) != ".json" || name == keyName("backend") {
			continue
		}
		info, err := s.root.Lstat(name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("state record must be a regular file")
		}
		file, err := s.root.Open(name)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return err
		}
		if err = visit(json.RawMessage(body)); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) Records() ([]json.RawMessage, error) {
	out := []json.RawMessage{}
	err := s.Walk(func(raw json.RawMessage) error { out = append(out, raw); return nil })
	return out, err
}
func (s *Store) Close() error { return s.root.Close() }

func (s *Store) Remove(key string) error { return s.root.Remove(keyName(key)) }
