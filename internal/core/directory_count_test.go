package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryCount(t *testing.T) {
	root := t.TempDir()
	count, err := CountDirectory(context.Background(), root, nil)
	if err != nil || count != (DirectoryCount{}) {
		t.Fatalf("empty directory: %+v %v", count, err)
	}
	put(t, filepath.Join(root, "nested", "deeper", "file.txt"), "file")
	put(t, filepath.Join(root, ".hidden"), "hidden")
	want := DirectoryCount{Folders: 2, Files: 2, Bytes: 10}
	if err := os.Symlink(root, filepath.Join(root, "cycle")); err == nil {
		want.Files++
	}
	if err := os.Symlink(filepath.Join(root, ".hidden"), filepath.Join(root, "file-link")); err == nil {
		want.Files++
	}
	var updates []DirectoryCount
	count, err = CountDirectory(context.Background(), root, func(c DirectoryCount) { updates = append(updates, c) })
	if err != nil || count != want || len(updates) != want.Files+want.Folders {
		t.Fatalf("recursive count: %+v, updates: %+v, error: %v", count, updates, err)
	}
	for i, c := range updates {
		if c.Files+c.Folders != i+1 {
			t.Fatalf("non-incremental progress: %+v", updates)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = CountDirectory(ctx, root, func(DirectoryCount) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	for _, path := range []string{"", filepath.Join(root, "missing"), filepath.Join(root, ".hidden")} {
		if _, err := CountDirectory(context.Background(), path, nil); err == nil {
			t.Fatalf("invalid directory accepted: %q", path)
		}
	}
}
