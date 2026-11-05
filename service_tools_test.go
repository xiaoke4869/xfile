package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"xfile/internal/core"
)

func TestPreviewTargetCache(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	s := newFileService()
	s.tasksPath = filepath.Join(root, "logs")
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(source, "结果")
	if err := os.Mkdir(output, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "old.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	c := core.Config{Sources: []core.Source{{Path: source}}, Target: output, Mode: "extract"}
	first, err := s.Preview(c, 1, false)
	if err != nil || first.FileCount != 1 {
		t.Fatalf("nested output scanned %+v %v", first, err)
	}
	c.Target = filepath.Join(root, "outside")
	second, err := s.Preview(c, 2, false)
	if err != nil || second.FileCount != 2 {
		t.Fatalf("source cache reused after target change %+v %v", second, err)
	}
}

func TestExtractionCreatesTargetOnlyWhenMoving(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	s := newFileService()
	s.tasksPath = filepath.Join(root, "logs")
	target := filepath.Join(source, "result-20261104")
	p, err := s.Preview(core.Config{Sources: []core.Source{{Path: source}}, Target: target, Mode: "extract"}, 1, true)
	if err != nil || p.FileCount != 1 || len(p.Issues) > 0 {
		t.Fatalf("preview %+v %v", p, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("preview created target", err)
	}
	if _, err := s.Start(p.ID, p.Revision); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for s.Task().Status == "running" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if task := s.Task(); task.Status != "succeeded" || task.Moved != 1 || task.Remaining != 0 {
		t.Fatalf("move failed %+v", task)
	}
	if data, err := os.ReadFile(filepath.Join(target, "a.txt")); err != nil || string(data) != "original" {
		t.Fatal("target content", string(data), err)
	}
	if _, err := os.Stat(filepath.Join(source, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("source remains", err)
	}
}
