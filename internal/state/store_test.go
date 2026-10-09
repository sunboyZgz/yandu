package state

import (
	"fmt"
	"testing"
	"yandu/internal/model"
)

func TestRollbackAndRestart(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Update(func(v *model.Snapshot) error { v.Revision = 42; return fmt.Errorf("abort") }); e == nil {
		t.Fatal("expected abort")
	}
	v, _ := s.Read()
	if v.Revision != 0 {
		t.Fatal("uncommitted state leaked")
	}
	s.Update(func(v *model.Snapshot) error { v.Revision = 7; return nil })
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	v, _ = s.Read()
	if v.Revision != 7 {
		t.Fatal("state did not survive restart")
	}
}
