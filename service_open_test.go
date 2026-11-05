package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLocalFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "中文 空格 & report.txt")
	if err := os.WriteFile(file, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	invalid := []string{"", "relative.txt", "https://example.com/file.txt", root, filepath.Join(root, "missing")}
	if err := os.Symlink(file, link); err == nil {
		invalid = append(invalid, link)
	} else {
		t.Logf("symlink creation unavailable: %v", err)
	}
	for _, path := range invalid {
		t.Run(path, func(t *testing.T) {
			called := false
			err := openLocalFile(path, func(string) error { called = true; return nil })
			if err == nil || called {
				t.Fatalf("invalid file reached launcher: %q, error: %v", path, err)
			}
		})
	}
	var opened string
	if err := openLocalFile(file, func(path string) error { opened = path; return nil }); err != nil || opened != file {
		t.Fatalf("file path changed: %q, error: %v", opened, err)
	}
	unavailable := errors.New("default application unavailable")
	if err := openLocalFile(file, func(string) error { return unavailable }); !errors.Is(err, unavailable) {
		t.Fatalf("launcher error lost: %v", err)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "original" {
		t.Fatalf("opening changed file: %q, %v", data, err)
	}
}
