package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserTransferAndCollision(t *testing.T) {
	root, err := Canonical(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "来源", "资料")
	put(t, filepath.Join(folder, "子目录", "1.txt"), "original")
	put(t, filepath.Join(folder, ".hidden"), "hidden")
	if err = os.Mkdir(filepath.Join(folder, "空目录"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "来源", "a.txt")
	put(t, file, "new")
	target := filepath.Join(root, "目标")
	put(t, filepath.Join(target, "资料", "old.txt"), "old")
	put(t, filepath.Join(target, "a.txt"), "old")
	result, err := TransferEntries(context.Background(), []string{filepath.Join(folder, "子目录"), folder, file}, target, false)
	if err != nil || len(result.Items) != 2 {
		t.Fatalf("copy %+v %v", result, err)
	}
	for _, item := range result.Items {
		if item.Error != "" {
			t.Fatal(item.Error)
		}
	}
	for path, expected := range map[string]string{filepath.Join(target, "资料__2", "子目录", "1.txt"): "original", filepath.Join(target, "资料__2", ".hidden"): "hidden", filepath.Join(target, "资料", "old.txt"): "old", filepath.Join(target, "a.txt"): "old", filepath.Join(target, "a__2.txt"): "new", file: "new"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != expected {
			t.Fatalf("%s: %q %v", path, data, err)
		}
	}
	if info, err := os.Stat(filepath.Join(target, "资料__2", "空目录")); err != nil || !info.IsDir() {
		t.Fatalf("empty dir %v", err)
	}
	same, err := TransferEntries(context.Background(), []string{file}, filepath.Dir(file), false)
	if err != nil || same.Items[0].Error != "" || filepath.Base(same.Items[0].Target) != "a__2.txt" {
		t.Fatalf("same directory copy %+v %v", same, err)
	}
	move, err := TransferEntries(context.Background(), []string{folder}, target, true)
	if err != nil || move.Items[0].Error != "" || filepath.Base(move.Items[0].Target) != "资料__3" {
		t.Fatalf("move %+v %v", move, err)
	}
	if _, err = os.Stat(folder); !os.IsNotExist(err) {
		t.Fatal("source directory remains", err)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= 16 && entry.Name()[:16] == ".xfile-transfer-" {
			t.Fatal("staging remains", entry.Name())
		}
	}
}
func TestBrowserTransferGuards(t *testing.T) {
	root, _ := Canonical(t.TempDir())
	source := filepath.Join(root, "source")
	put(t, filepath.Join(source, "a.txt"), "a")
	child := filepath.Join(source, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := TransferEntries(context.Background(), []string{source}, child, false); err == nil {
		t.Fatal("accepted descendant target")
	}
	same, err := TransferEntries(context.Background(), []string{source}, root, true)
	if err != nil || same.Items[0].Error == "" {
		t.Fatalf("same parent move %+v %v", same, err)
	}
	target := filepath.Join(root, "target")
	if err = os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := TransferEntries(ctx, []string{source}, target, false)
	if err != nil || result.Items[0].Error == "" {
		t.Fatalf("cancellation %+v %v", result, err)
	}
	if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
		t.Fatal("cancelled copy wrote files", entries, err)
	}
	if _, err = SelectedPaths([]string{string(filepath.Separator)}); err == nil {
		t.Fatal("accepted filesystem root")
	}
}
func TestBrowserVerifiedCopyMoveAndSourceChanges(t *testing.T) {
	root, _ := Canonical(t.TempDir())
	source := filepath.Join(root, "source")
	put(t, filepath.Join(source, "a.txt"), "first")
	put(t, filepath.Join(source, "empty.txt"), "")
	if err := os.Mkdir(filepath.Join(source, "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	// This is the same verified path used after a cross-volume rename fails.
	destination, err := transferByCopy(context.Background(), source, target, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("copy-based move retained source", err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "a.txt"))
	if err != nil || string(data) != "first" {
		t.Fatal("bad content", string(data), err)
	}
	staging := filepath.Join(root, "snapshot")
	nodes, err := copyEntry(context.Background(), destination, staging)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.Stat(filepath.Join(destination, "a.txt"))
	put(t, filepath.Join(destination, "a.txt"), "other")
	if err = os.Chtimes(filepath.Join(destination, "a.txt"), original.ModTime(), original.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err = verifyEntry(context.Background(), destination, nodes); err == nil {
		t.Fatal("missed changed content with equal size and time")
	}
	data, err = os.ReadFile(filepath.Join(staging, "a.txt"))
	if err != nil || string(data) != "first" {
		t.Fatal("verified copy altered", string(data), err)
	}
}
func TestBrowserDirectoryLinksAreNotFollowed(t *testing.T) {
	root, _ := Canonical(t.TempDir())
	source := filepath.Join(root, "source")
	put(t, filepath.Join(source, "a.txt"), "a")
	outside := filepath.Join(root, "outside")
	put(t, filepath.Join(outside, "keep.txt"), "keep")
	link := filepath.Join(source, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlinks unavailable", err)
	}
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	result, err := TransferEntries(context.Background(), []string{source}, target, false)
	if err != nil || result.Items[0].Error != "" {
		t.Fatalf("copy link %+v %v", result, err)
	}
	copied := filepath.Join(target, "source", "link")
	if actual, err := os.Readlink(copied); err != nil || actual != outside {
		t.Fatal("link followed", actual, err)
	}
	if _, err = SelectedPaths([]string{link}); err == nil {
		t.Fatal("top level link accepted")
	}
	moved := filepath.Join(root, "moved")
	if err = os.Mkdir(moved, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = transferByCopy(context.Background(), source, moved, true); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatal("link target changed", string(data), err)
	}
}
