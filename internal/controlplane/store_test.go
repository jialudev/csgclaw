package controlplane

import (
	"os"
	"testing"
)

func TestDurableIdentityAndAtomicRecordReplacement(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := s.ID
	if err = s.Write("agent/../scope", map[string]int{"version": 1}); err != nil {
		t.Fatal(err)
	}
	// JSON encoding fails before replacing the acknowledged record.
	if err = s.Write("agent/../scope", make(chan int)); err == nil {
		t.Fatal("invalid record accepted")
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var record map[string]int
	if err = s.Read("agent/../scope", &record); err != nil || record["version"] != 1 || s.ID != id {
		t.Fatalf("recovery: %+v %v", record, err)
	}
	if err = s.WriteBytes("../outside", []byte("x")); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err = os.Stat(dir + "/../outside"); !os.IsNotExist(err) {
		t.Fatal("wrote outside root")
	}
}
