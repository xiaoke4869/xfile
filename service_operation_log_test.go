package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"xfile/internal/core"
)

func TestFileOperationJournalAndReadOnlyActions(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	for _, path := range []string{source, target} {
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(source, "original.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	// Real file operations, followed by a new service instance reading the persisted records.
	if _, err := s.TransferEntries([]string{file}, target, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransferEntries([]string{file}, target, true); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(target, "original.txt")
	if _, err := s.RenameEntries([]string{copied}, core.RenameConfig{Mode: "single", Name: "renamed.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFolder(source, "new-folder"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteEntries([]string{filepath.Join(root, "missing")}); err == nil {
		t.Fatal("invalid delete accepted")
	}
	renamed := filepath.Join(target, "renamed.txt")
	if _, err := s.fileOperation("delete", []string{renamed, source}, "", func() (core.OperationResult, error) {
		return trashEntries([]string{renamed, source}, func(path string) error {
			if path == source {
				return errors.New("trash unavailable")
			}
			return os.Rename(path, filepath.Join(root, "mock-trash"))
		}), nil
	}); err != nil {
		t.Fatal(err)
	}
	s = &FileService{tasksPath: s.tasksPath}
	tasks, err := s.Tasks()
	if err != nil || len(tasks) != 6 {
		t.Fatalf("history: %+v %v", tasks, err)
	}
	seen := map[string]int{}
	for _, task := range tasks {
		seen[task.Operation]++
		if task.Items != nil {
			t.Fatal("list unnecessarily contains item details")
		}
		detail, err := s.InspectTask(task.ID)
		if err != nil || len(detail.Items) == 0 || detail.Started.IsZero() || detail.Finished.IsZero() {
			t.Fatalf("detail: %+v %v", detail, err)
		}
		if _, err := os.Stat(filepath.Join(detail.Directory, "report.md")); err != nil {
			t.Fatal(err)
		}
		if detail.Status == "partial_failed" && (detail.Moved != 1 || detail.Total != 2 || detail.Items[1].Error == "") {
			t.Fatalf("partial result lost: %+v", detail)
		}
		if detail.Operation == "copy" && (detail.Items[0].Source != file || detail.Items[0].Target != copied) {
			t.Fatalf("copy paths lost: %+v", detail)
		}
		if _, err := s.PrepareRecovery(task.ID, 1); err == nil {
			t.Fatal("ordinary operation accepted as merge recovery")
		}
	}
	if seen["copy"] != 1 || seen["move"] != 1 || seen["rename"] != 1 || seen["delete"] != 2 || seen["create_folder"] != 1 {
		t.Fatal(seen)
	}
	if _, err = s.List(source); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EntryInfo(source); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CountDirectory(context.Background(), source, "directory-count:test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DirectoryText(source); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PreviewRename([]string{source}, core.RenameConfig{Mode: "single", Name: "preview-only"}); err != nil {
		t.Fatal(err)
	}
	after, err := s.Tasks()
	if err != nil || len(after) != len(tasks) {
		t.Fatalf("read-only actions created records: %+v %v", after, err)
	}
}

func TestLiveFileOperationSummaryAndJournal(t *testing.T) {
	root := t.TempDir()
	s := &FileService{tasksPath: filepath.Join(root, "logs"), task: core.Task{ID: "old-merge", Status: "succeeded"}}
	_, err := s.fileOperation("move", []string{"first", "second"}, "target", func() (core.OperationResult, error) {
		live := s.Task()
		if live.ID == "" || live.ID == "old-merge" || live.Operation != "move" || live.Status != "running" || live.Total != 2 || live.Items != nil {
			t.Fatalf("wrong live operation: %+v", live)
		}
		stored, err := s.InspectTask(live.ID)
		if err != nil || stored.Status != "running" || len(stored.Items) != 2 {
			t.Fatalf("live journal: %+v %v", stored, err)
		}
		return core.OperationResult{Items: []core.OperationItem{{Source: "first", Target: "target/first"}, {Source: "second", Error: "permission denied"}}}, nil
	})
	if err != nil || s.fileOperationRunning || s.fileTask.Status != "partial_failed" || s.fileTask.Moved != 1 || s.fileTask.Items != nil {
		t.Fatalf("terminal summary: %+v %v", s.fileTask, err)
	}
	detail, err := s.InspectTask(s.fileTask.ID)
	if err != nil || len(detail.Items) != 2 || detail.Items[1].Error != "permission denied" || detail.Status != s.fileTask.Status {
		t.Fatalf("persisted result differs from live result: %+v %v", detail, err)
	}
}

func TestInterruptedOperationAndUnwritableJournal(t *testing.T) {
	root := t.TempDir()
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	dir := filepath.Join(s.tasksPath, "interrupted")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	stored := core.Task{ID: "interrupted", Operation: "rename", Status: "running", Total: 1, Items: []core.OperationItem{{Source: "old.txt", Target: "new.txt"}}}
	if err := core.AtomicJSON(filepath.Join(dir, "summary.json"), stored); err != nil {
		t.Fatal(err)
	}
	detail, err := s.InspectTask(stored.ID)
	if err != nil || detail.Status != "needs_review" || detail.Unknown != 1 || detail.Items[0].Target != "new.txt" {
		t.Fatalf("interrupted: %+v %v", detail, err)
	}
	// Viewing the interrupted record must not rewrite it or claim recovery.
	b, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = core.AtomicJSON(filepath.Join(root, "blocked"), stored); err != nil {
		t.Fatal(err)
	}
	s.tasksPath = filepath.Join(root, "blocked")
	ran := false
	if _, err = s.fileOperation("copy", []string{"source"}, "target", func() (core.OperationResult, error) { ran = true; return core.OperationResult{}, nil }); err == nil || ran || s.fileOperationRunning {
		t.Fatalf("operation ran without journal: %v %v", ran, err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil || string(after) != string(b) {
		t.Fatal("inspection rewrote the journal")
	}
}

func TestDeleteAndClearTaskRecordsOnly(t *testing.T) {
	root := t.TempDir()
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	file := filepath.Join(root, "business-file.txt")
	if err := os.WriteFile(file, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		dir := filepath.Join(s.tasksPath, id)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := core.AtomicJSON(filepath.Join(dir, "summary.json"), core.Task{ID: id, Status: "succeeded", Items: []core.OperationItem{{Source: file, Target: file}}}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "report.md"), []byte("report"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Unrelated folders and symlinks must never be followed or removed by clear-all.
	unrelated := filepath.Join(s.tasksPath, "unrelated")
	if err := os.Mkdir(unrelated, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unrelated, "keep.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(s.tasksPath, "linked")
	if err := os.Symlink(root, link); err == nil {
		if err := s.DeleteTask("linked"); err == nil {
			t.Fatal("symlink accepted as record")
		}
	}
	for _, id := range []string{"", ".", "..", "../business-file.txt", root} {
		if err := s.DeleteTask(id); err == nil {
			t.Fatalf("invalid ID accepted: %q", id)
		}
	}
	for _, ordinary := range []bool{false, true} {
		s.taskRunning, s.fileOperationRunning = !ordinary, ordinary
		if err := s.DeleteTask("one"); err == nil {
			t.Fatal("deleted record during active operation")
		}
		if err := s.ClearTasks(); err == nil {
			t.Fatal("cleared records during active operation")
		}
	}
	s.taskRunning, s.fileOperationRunning = false, false
	s.task = core.Task{ID: "one"}
	s.plan = &core.Plan{RecoveryFrom: "one"}
	if err := s.DeleteTask("one"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.tasksPath, "one")); !os.IsNotExist(err) {
		t.Fatal("record files retained", err)
	}
	if s.task.ID != "" || s.plan != nil {
		t.Fatal("deleted task still available in memory")
	}
	remaining, err := s.Tasks()
	if err != nil || len(remaining) != 1 || remaining[0].ID != "two" {
		t.Fatalf("single deletion affected other records: %+v %v", remaining, err)
	}
	if err := s.ClearTasks(); err != nil {
		t.Fatal(err)
	}
	remaining, err = s.Tasks()
	if err != nil || len(remaining) != 0 {
		t.Fatalf("clear retained records: %+v %v", remaining, err)
	}
	for _, path := range []string{file, filepath.Join(unrelated, "keep.txt")} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "keep" {
			t.Fatalf("record deletion changed unrelated files: %s %v", path, err)
		}
	}
	if err := s.ClearTasks(); err != nil {
		t.Fatal("clearing empty history failed", err)
	}
}
