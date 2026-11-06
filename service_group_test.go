package main

import (
	"os"
	"path/filepath"
	"testing"
	"xfile/internal/core"
)

func TestGroupEntriesContentCollisionsAndJournal(t *testing.T) {
	root, err := core.Canonical(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	directory := filepath.Join(root, "files")
	folder := filepath.Join(directory, "资料")
	empty := filepath.Join(folder, "空目录")
	if err = os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{
		filepath.Join(directory, "中文 & report.txt"): "report content",
		filepath.Join(folder, ".hidden"):            "hidden content",
		filepath.Join(folder, "inside.txt"):         "nested content",
		filepath.Join(directory, "用所选项目新建的文件夹"):     "existing file",
	}
	for path, content := range contents {
		if err = os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	occupied := filepath.Join(directory, "用所选项目新建的文件夹_2")
	if err = os.Mkdir(occupied, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "中文 & report.txt")
	// A parent and its selected child must move only once, regardless of order.
	result, err := s.GroupEntries([]string{filepath.Join(folder, "inside.txt"), folder, file}, directory)
	if err != nil || result.Folder != filepath.Join(directory, "用所选项目新建的文件夹_3") || len(result.Items) != 2 {
		t.Fatalf("group: %+v %v", result, err)
	}
	for _, item := range result.Items {
		if item.Error != "" {
			t.Fatal(item)
		}
		if _, err = os.Lstat(item.Source); !os.IsNotExist(err) {
			t.Fatalf("source remains: %s %v", item.Source, err)
		}
	}
	for original, expected := range contents {
		path := original
		if original != filepath.Join(directory, "用所选项目新建的文件夹") {
			relative, _ := filepath.Rel(directory, original)
			path = filepath.Join(result.Folder, relative)
		}
		if content, err := os.ReadFile(path); err != nil || string(content) != expected {
			t.Fatalf("content changed: %s %q %v", path, content, err)
		}
	}
	for _, path := range []string{occupied, filepath.Join(result.Folder, "资料", "空目录")} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("directory lost: %s %v", path, err)
		}
	}
	// A new service instance reads exactly one persisted composite operation.
	reader := &FileService{tasksPath: s.tasksPath}
	tasks, err := reader.Tasks()
	if err != nil || len(tasks) != 1 || tasks[0].Operation != "group" || tasks[0].Status != "succeeded" || tasks[0].Total != 2 || tasks[0].Moved != 2 {
		t.Fatalf("journal: %+v %v", tasks, err)
	}
	detail, err := reader.InspectTask(tasks[0].ID)
	if err != nil || len(detail.Items) != 2 || detail.Items[0].Target != result.Items[0].Target {
		t.Fatalf("detail: %+v %v", detail, err)
	}
	if _, err = os.Stat(filepath.Join(detail.Directory, "report.md")); err != nil {
		t.Fatal(err)
	}
}

func TestGroupEntriesSameNamesAcrossExpandedDirectories(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	var paths []string
	for _, branch := range []string{"left", "right"} {
		dir := filepath.Join(root, "files", branch)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "same.txt")
		if err := os.WriteFile(path, []byte(branch), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	result, err := s.GroupEntries(paths, filepath.Join(root, "files"))
	if err != nil || len(result.Items) != 2 {
		t.Fatalf("group: %+v %v", result, err)
	}
	for name, expected := range map[string]string{"same.txt": "left", "same__2.txt": "right"} {
		if data, err := os.ReadFile(filepath.Join(result.Folder, name)); err != nil || string(data) != expected {
			t.Fatalf("collision lost content: %s %q %v", name, data, err)
		}
	}
}

func TestGroupEntriesDirectoriesIncludingDefaultName(t *testing.T) {
	root, _ := core.Canonical(t.TempDir())
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	directory := filepath.Join(root, "files")
	paths := []string{filepath.Join(directory, "用所选项目新建的文件夹"), filepath.Join(directory, "另一个目录")}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Join(path, "空目录"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	result, err := s.GroupEntries(paths, directory)
	if err != nil || result.Folder != paths[0]+"_2" || len(result.Items) != 2 {
		t.Fatalf("directory grouping: %+v %v", result, err)
	}
	for _, item := range result.Items {
		if item.Error != "" {
			t.Fatal(item)
		}
		if info, err := os.Stat(filepath.Join(item.Target, "空目录")); err != nil || !info.IsDir() {
			t.Fatalf("empty directory lost: %+v %v", item, err)
		}
	}
}

func TestGroupEntriesGuardsDoNotMoveSources(t *testing.T) {
	for _, scenario := range []string{"single", "missing", "outside", "source-is-current", "log-source", "log-target", "journal-unavailable", "busy", "non-directory", "denied", "link"} {
		t.Run(scenario, func(t *testing.T) {
			root, _ := core.Canonical(t.TempDir())
			directory := filepath.Join(root, "files")
			if err := os.Mkdir(directory, 0755); err != nil {
				t.Fatal(err)
			}
			s := &FileService{tasksPath: filepath.Join(root, "logs")}
			one, two := filepath.Join(directory, "a.txt"), filepath.Join(directory, "b.txt")
			for _, path := range []string{one, two} {
				if err := os.WriteFile(path, []byte("keep"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			paths, target := []string{one, two}, directory
			switch scenario {
			case "single":
				paths = paths[:1]
			case "missing":
				paths[1] = filepath.Join(directory, "missing")
			case "outside":
				target = filepath.Join(root, "other")
				if err := os.Mkdir(target, 0755); err != nil {
					t.Fatal(err)
				}
			case "source-is-current":
				paths[1] = directory
			case "log-source":
				if err := os.Mkdir(s.tasksPath, 0700); err != nil {
					t.Fatal(err)
				}
				paths[1] = s.tasksPath
			case "log-target":
				target = s.tasksPath
			case "journal-unavailable":
				s.tasksPath = filepath.Join(root, "blocked")
				if err := os.WriteFile(s.tasksPath, []byte("keep log blocker"), 0600); err != nil {
					t.Fatal(err)
				}
			case "busy":
				s.fileOperationRunning = true
			case "non-directory":
				target = one
			case "denied":
				if os.Geteuid() == 0 {
					t.Skip("root bypasses filesystem permissions")
				}
				if err := os.Chmod(directory, 0555); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(directory, 0755)
			case "link":
				link := filepath.Join(directory, "link")
				if err := os.Symlink(one, link); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				paths[1] = link
			}
			result, err := s.GroupEntries(paths, target)
			if err == nil || result.Folder != "" {
				t.Fatalf("unsafe grouping accepted: %+v %v", result, err)
			}
			for _, path := range []string{one, two} {
				if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
					t.Fatalf("source changed: %s %q %v", path, data, err)
				}
			}
			if _, err = os.Stat(filepath.Join(directory, "用所选项目新建的文件夹")); !os.IsNotExist(err) {
				t.Fatalf("failed validation created a folder: %v", err)
			}
		})
	}
}

func TestGroupEntriesPartialFailureKeepsResultsAndJournal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	root, _ := core.Canonical(t.TempDir())
	s := &FileService{tasksPath: filepath.Join(root, "logs")}
	directory := filepath.Join(root, "files")
	locked := filepath.Join(directory, "locked")
	if err := os.MkdirAll(locked, 0755); err != nil {
		t.Fatal(err)
	}
	one, two := filepath.Join(directory, "good.txt"), filepath.Join(locked, "denied.txt")
	for _, path := range []string{one, two} {
		if err := os.WriteFile(path, []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(locked, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0755)
	result, err := s.GroupEntries([]string{one, two}, directory)
	if err != nil || len(result.Items) != 2 || result.Items[0].Error != "" || result.Items[1].Error == "" || result.Folder == "" {
		t.Fatalf("partial result: %+v %v", result, err)
	}
	for _, path := range []string{filepath.Join(result.Folder, "good.txt"), two} {
		if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
			t.Fatalf("partial result lost data: %s %q %v", path, data, err)
		}
	}
	if _, err = os.Lstat(one); !os.IsNotExist(err) {
		t.Fatalf("successful move was undone: %v", err)
	}
	detail, err := s.InspectTask(s.fileTask.ID)
	if err != nil || detail.Status != "partial_failed" || detail.Moved != 1 || detail.Total != 2 || detail.Items[1].Error == "" || detail.Items[1].Target != result.Folder {
		t.Fatalf("partial journal: %+v %v", detail, err)
	}
}
