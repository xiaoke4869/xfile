package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"xfile/internal/core"
)

func TestDirectoryMeasurementFailuresStayInTaskResults(t *testing.T) {
	root := t.TempDir()
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	result := s.MeasureDirectory(context.Background(), root, "directory-count:test")
	if result.Error != "" || result.Bytes != 7 || result.Files != 1 {
		t.Fatalf("measurement: %+v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, input := range []struct {
		ctx         context.Context
		path, event string
	}{{ctx, root, "directory-count:cancel"}, {context.Background(), file, "directory-count:file"}, {context.Background(), filepath.Join(root, "missing"), "directory-count:missing"}, {context.Background(), root, "invalid"}} {
		result = s.MeasureDirectory(input.ctx, input.path, input.event)
		if result.Error == "" {
			t.Fatalf("failure was lost: %+v", input)
		}
		data, err := json.Marshal(result)
		var payload struct {
			Bytes int64  `json:"bytes"`
			Error string `json:"error"`
		}
		if err != nil || json.Unmarshal(data, &payload) != nil || payload.Error != result.Error || payload.Bytes != result.Bytes {
			t.Fatalf("failure cannot be transported as task data: %s %v", data, err)
		}
	}
	denied := filepath.Join(root, "denied")
	if err := os.Mkdir(denied, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(denied, 0700)
	if _, err := os.ReadDir(denied); err != nil {
		if result := s.MeasureDirectory(context.Background(), denied, "directory-count:denied"); result.Error == "" {
			t.Fatalf("permission failure was lost: %+v", result)
		}
	}
	if _, err := os.Stat(s.tasksPath); !os.IsNotExist(err) {
		t.Fatalf("read-only measurement created logs: %v", err)
	}
}

func TestBrowserOperationsProtectLogsAndSerialize(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	for _, p := range []string{source, target} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	s := newFileService()
	s.tasksPath = filepath.Join(root, "logs")
	if err := os.Mkdir(s.tasksPath, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransferEntries([]string{s.tasksPath}, target, false); err == nil {
		t.Fatal("log copy accepted")
	}
	if _, err := s.DeleteEntries([]string{root}); err == nil {
		t.Fatal("log ancestor delete accepted")
	}
	if _, err := s.TransferEntries([]string{source}, s.tasksPath, false); err == nil {
		t.Fatal("log target accepted")
	}
	s.taskRunning = true
	if _, err := s.DeleteEntries([]string{source}); err == nil {
		t.Fatal("delete concurrent with move")
	}
	s.taskRunning = false
	if err := s.beginFileOperation(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start("missing", 1); err == nil {
		t.Fatal("merge started during file operation")
	}
	if _, err := s.TransferEntries([]string{source}, target, false); err == nil {
		t.Fatal("concurrent copy accepted")
	}
	s.finishFileOperation()
	result, err := s.TransferEntries([]string{source}, target, false)
	if err != nil || len(result.Items) != 1 || result.Items[0].Error != "" {
		t.Fatalf("copy %+v %v", result, err)
	}
	if s.fileOperationRunning {
		t.Fatal("busy state retained")
	}
	// Copying to an ancestor of the log folder is allowed: entries are never merged.
	sibling := filepath.Join(root, "elsewhere")
	if err = os.Mkdir(sibling, 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(sibling, "b.txt")
	if err = os.WriteFile(p, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	if result, err = s.TransferEntries([]string{p}, root, false); err != nil || result.Items[0].Error != "" {
		t.Fatalf("ancestor target %+v %v", result, err)
	}
}
func TestBrowserTrashReportsPerItemWithoutPermanentFallback(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	trash := filepath.Join(root, "trash")
	for _, p := range []string{one, two} {
		if err := os.WriteFile(p, []byte("original"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	result := trashEntries([]string{one, two}, func(p string) error {
		if p == one {
			return errors.New("recycle unavailable")
		}
		return os.Rename(p, trash)
	})
	if len(result.Items) != 2 || result.Items[0].Error == "" || result.Items[1].Error != "" {
		t.Fatalf("trash result %+v", result)
	}
	for _, p := range []string{one, trash} {
		if data, err := os.ReadFile(p); err != nil || string(data) != "original" {
			t.Fatal("content not retained", p, string(data), err)
		}
	}
}

func TestRenameServiceProtectionAndPreviewLimit(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	s := newFileService()
	s.tasksPath = filepath.Join(root, "logs")
	if err := os.Mkdir(s.tasksPath, 0755); err != nil {
		t.Fatal(err)
	}
	config := core.RenameConfig{Mode: "single", Name: "renamed"}
	if _, err := s.PreviewRename([]string{s.tasksPath}, config); err == nil {
		t.Fatal("log preview accepted")
	}
	if _, err := s.RenameEntries([]string{root}, config); err == nil {
		t.Fatal("log ancestor rename accepted")
	}
	paths := []string{}
	for i := 0; i < 12; i++ {
		path := filepath.Join(root, string(rune('a'+i))+".txt")
		if err := os.WriteFile(path, []byte("unchanged"), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	config = core.RenameConfig{Mode: "format", Format: "壁纸", Position: "after", Start: 9}
	p, err := s.PreviewRename(paths, config)
	if err != nil || p.Total != 12 || p.Changed != 12 || len(p.Items) != 10 {
		t.Fatalf("preview %+v %v", p, err)
	}
	s.taskRunning = true
	if _, err = s.RenameEntries(paths, config); err == nil {
		t.Fatal("concurrent rename accepted")
	}
	s.taskRunning = false
	// A conflict beyond the displayed ten rows must still block the whole batch.
	conflict := filepath.Join(root, "壁纸20.txt")
	if err = os.WriteFile(conflict, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err = s.PreviewRename(paths, config)
	if err != nil || len(p.Items) != 10 || len(p.Issues) == 0 {
		t.Fatalf("hidden conflict %+v %v", p, err)
	}
	if _, err = s.RenameEntries(paths, config); err == nil {
		t.Fatal("hidden conflict accepted")
	}
	for _, path := range paths {
		if _, err = os.Stat(path); err != nil {
			t.Fatal("conflicted batch changed source", err)
		}
	}
	if err = os.Remove(conflict); err != nil {
		t.Fatal(err)
	}
	result, err := s.RenameEntries(paths, config)
	if err != nil || len(result.Items) != 12 || s.fileOperationRunning {
		t.Fatalf("rename %+v %v", result, err)
	}
	for _, item := range result.Items {
		if item.Error != "" {
			t.Fatal(item.Error)
		}
		if _, err = os.Stat(item.Target); err != nil {
			t.Fatal(err)
		}
	}
}
