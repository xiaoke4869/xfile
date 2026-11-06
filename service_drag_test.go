package main

import (
	"os"
	"path/filepath"
	"testing"
	"xfile/internal/core"
)

func TestGroupFilesDropContentPlacementAndJournal(t *testing.T) {
	for _, count := range []int{1, 3} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			root, _ := core.Canonical(t.TempDir())
			s := &FileService{tasksPath: filepath.Join(root, "logs")}
			destination := filepath.Join(root, "expanded", "target")
			if err := os.MkdirAll(destination, 0755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(destination, "接收文件.txt")
			contents := map[string]string{target: "target content"}
			var paths []string
			for i := 0; i < count; i++ {
				dir := filepath.Join(root, "source", string(rune('a'+i)))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "同名.txt")
				paths = append(paths, path)
				contents[path] = "source " + string(rune('a'+i))
			}
			for path, content := range contents {
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			base := filepath.Join(destination, "合成的文件夹")
			if err := os.WriteFile(base, []byte("occupied"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(base+"_2", 0755); err != nil {
				t.Fatal(err)
			}
			result, err := s.GroupFiles(paths, target)
			if err != nil || result.Folder != base+"_3" || len(result.Items) != count+1 {
				t.Fatalf("drop result: %+v %v", result, err)
			}
			for _, item := range result.Items {
				data, err := os.ReadFile(item.Target)
				if item.Error != "" || err != nil || string(data) != contents[item.Source] {
					t.Fatalf("content: %+v %q %v", item, data, err)
				}
				if _, err := os.Stat(item.Source); !os.IsNotExist(err) {
					t.Fatalf("source remains: %s %v", item.Source, err)
				}
			}
			if data, err := os.ReadFile(base); err != nil || string(data) != "occupied" {
				t.Fatalf("overwrote occupied name: %q %v", data, err)
			}
			reader := &FileService{tasksPath: s.tasksPath}
			tasks, err := reader.Tasks()
			if err != nil || len(tasks) != 1 || tasks[0].Operation != "group" || tasks[0].Status != "succeeded" || tasks[0].Total != count+1 {
				t.Fatalf("journal: %+v %v", tasks, err)
			}
			detail, err := reader.InspectTask(tasks[0].ID)
			if err != nil || len(detail.Items) != count+1 || detail.Items[count].Source != target {
				t.Fatalf("target file missing from journal: %+v %v", detail, err)
			}
		})
	}
}

func TestGroupFilesRejectsInvalidDropBeforeCreation(t *testing.T) {
	for _, scenario := range []string{"empty", "self", "duplicate", "source-directory", "target-directory", "missing", "link", "log-source", "log-target", "log-write", "busy"} {
		t.Run(scenario, func(t *testing.T) {
			root, _ := core.Canonical(t.TempDir())
			s := &FileService{tasksPath: filepath.Join(root, "logs")}
			dir := filepath.Join(root, "files")
			if err := os.Mkdir(dir, 0755); err != nil {
				t.Fatal(err)
			}
			file, target := filepath.Join(dir, "source.txt"), filepath.Join(dir, "target.txt")
			for _, path := range []string{file, target} {
				if err := os.WriteFile(path, []byte(path), 0644); err != nil {
					t.Fatal(err)
				}
			}
			paths := []string{file}
			switch scenario {
			case "empty":
				paths = nil
			case "self":
				target = file
			case "duplicate":
				paths = append(paths, file)
			case "source-directory":
				paths = []string{dir}
			case "target-directory":
				target = dir
			case "missing":
				paths = []string{filepath.Join(dir, "missing")}
			case "link":
				link := filepath.Join(dir, "link")
				if err := os.Symlink(file, link); err != nil {
					t.Fatal(err)
				}
				paths = []string{link}
			case "log-source":
				s.tasksPath = dir
			case "log-target":
				s.tasksPath = target
			case "log-write":
				s.tasksPath = filepath.Join(file, "logs")
			case "busy":
				s.fileOperationRunning = true
			}
			result, err := s.GroupFiles(paths, target)
			if err == nil || result.Folder != "" {
				t.Fatalf("invalid drop created folder: %+v %v", result, err)
			}
			for _, path := range []string{file, filepath.Join(dir, "target.txt")} {
				if data, err := os.ReadFile(path); err != nil || string(data) != path {
					t.Fatalf("source changed: %s %q %v", path, data, err)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "合成的文件夹")); !os.IsNotExist(err) {
				t.Fatalf("folder created on failure: %v", err)
			}
		})
	}
}
