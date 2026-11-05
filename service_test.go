package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"xfile/internal/core"
)

func TestServicePreviewVersionAndStalePlan(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "1.2 检验资料")
	if e := os.MkdirAll(source, 0755); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(source, "a.txt")
	if e := os.WriteFile(file, []byte("original"), 0644); e != nil {
		t.Fatal(e)
	}
	s := newFileService()
	s.tasksPath = filepath.Join(root, "logs")
	c := core.Config{Sources: []core.Source{{Path: source, Depth: 0}}, Target: filepath.Join(root, "target"), Rule: "numeric", Prefix: true}
	p, e := s.Preview(c, 1, true)
	if e != nil || p.FileCount != 1 {
		t.Fatalf("preview %v %v", p, e)
	}
	c.Prefix = false
	latest, e := s.Preview(c, 2, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Start(p.ID, 1); e == nil {
		t.Fatal("stale preview accepted")
	}
	if e = os.WriteFile(file, []byte("changed content"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Start(latest.ID, 2); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s.Task().Status != "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if s.Task().Status != "stale_plan" {
		t.Fatalf("changed source not detected: %+v", s.Task())
	}
	b, e := os.ReadFile(file)
	if e != nil || string(b) != "changed content" {
		t.Fatal("stale execution changed source")
	}
}
func TestTaskLogsCannotBeMoved(t *testing.T) {
	root := t.TempDir()
	s := newFileService()
	s.tasksPath = filepath.Join(root, "source", "logs")
	if e := os.MkdirAll(s.tasksPath, 0755); e != nil {
		t.Fatal(e)
	}
	_, e := s.Preview(core.Config{Sources: []core.Source{{Path: filepath.Join(root, "source")}}, Target: filepath.Join(root, "target")}, 1, true)
	if e == nil {
		t.Fatal("source contains task logs")
	}
}
