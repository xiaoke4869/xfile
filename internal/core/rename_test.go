package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameRules(t *testing.T) {
	for _, tc := range []struct {
		name   string
		dir    bool
		index  int
		config RenameConfig
		want   string
	}{
		{"你好.txt", false, 0, RenameConfig{Mode: "format", Format: "<原文件名>-", Position: "after", Start: 1}, "你好-1.txt"},
		{"9921.md", false, 1, RenameConfig{Mode: "format", Format: "<原文件名>-", Position: "after", Start: 1}, "9921-2.md"},
		{"a.jpg", false, 0, RenameConfig{Mode: "format", Format: "壁纸", Position: "after", Start: 9}, "壁纸9.jpg"},
		{"b.jpg", false, 1, RenameConfig{Mode: "format", Format: "壁纸", Position: "after", Start: 9}, "壁纸10.jpg"},
		{"a.tar.gz", false, 2, RenameConfig{Mode: "format", Format: "<原文件名>", Position: "before", Start: 0}, "2a.tar.gz"},
		{"a.a.txt", false, 0, RenameConfig{Mode: "replace", Find: "a", Replace: "$1"}, "$1.$1.txt"},
		{"hello.txt", false, 0, RenameConfig{Mode: "replace", Find: "hello", Replace: ""}, ".txt"},
		{"hello.txt", false, 0, RenameConfig{Mode: "add", Text: "新", Position: "before"}, "新hello.txt"},
		{"hello.txt", false, 0, RenameConfig{Mode: "add", Text: "新", Position: "after"}, "hello新.txt"},
		{"目录.v1", true, 0, RenameConfig{Mode: "add", Text: "新", Position: "after"}, "目录.v1新"},
		{".env", false, 0, RenameConfig{Mode: "add", Text: "新", Position: "after"}, ".env新"},
		{".config.txt", false, 0, RenameConfig{Mode: "add", Text: "新", Position: "after"}, ".config新.txt"},
		{"old.txt", false, 0, RenameConfig{Mode: "single", Name: "new.md"}, "new.md"},
	} {
		if got := renamedName(tc.name, tc.dir, tc.index, tc.config); got != tc.want {
			t.Errorf("%s: %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestRenamePreviewExecutionAndNestedSelections(t *testing.T) {
	root, _ := Canonical(t.TempDir())
	parent := filepath.Join(root, "目录.v1")
	child := filepath.Join(parent, "你好.txt")
	put(t, child, "original")
	if err := os.Mkdir(filepath.Join(parent, "空目录"), 0755); err != nil {
		t.Fatal(err)
	}
	c := RenameConfig{Mode: "format", Format: "<原文件名>-", Position: "after", Start: 1}
	p, err := PlanRenames([]string{parent, child}, c)
	if err != nil || len(p.Issues) != 0 || p.Changed != 2 {
		t.Fatalf("preview %+v %v", p, err)
	}
	if _, err = os.Stat(child); err != nil {
		t.Fatal("preview modified source", err)
	}
	r, err := RenameEntries([]string{parent, child}, c)
	if err != nil || len(r.Items) != 2 {
		t.Fatalf("rename %+v %v", r, err)
	}
	for _, item := range r.Items {
		if item.Error != "" {
			t.Fatal(item.Error)
		}
		if _, err = os.Stat(item.Target); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "目录.v1-1", "你好-2.txt"))
	if err != nil || string(data) != "original" {
		t.Fatal("content changed", string(data), err)
	}
	if _, err = os.Stat(filepath.Join(root, "目录.v1-1", "空目录")); err != nil {
		t.Fatal(err)
	}
}

func TestRenameGuardsAndNoOverwrite(t *testing.T) {
	root, _ := Canonical(t.TempDir())
	a, b := filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")
	put(t, a, "a")
	put(t, b, "b")
	for _, c := range []RenameConfig{
		{Mode: "single", Name: "b.txt"}, {Mode: "single", Name: "../escape"}, {Mode: "single", Name: "CON"}, {Mode: "single", Name: ""},
		{Mode: "replace"}, {Mode: "format", Format: "x", Position: "after", Start: -1}, {Mode: "format", Format: "x", Position: "after", Start: 9007199254740992},
		{Mode: "add", Text: "x", Position: "invalid"},
	} {
		if _, err := RenameEntries([]string{a}, c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
	if _, err := RenameEntries([]string{a, b}, RenameConfig{Mode: "replace", Find: "b", Replace: "a"}); err == nil {
		t.Fatal("collision accepted")
	}
	for p, want := range map[string]string{a: "a", b: "b"} {
		data, err := os.ReadFile(p)
		if err != nil || string(data) != want {
			t.Fatal("source changed", p, err)
		}
	}
	p, err := PlanRenames([]string{a}, RenameConfig{Mode: "single", Name: "a.txt"})
	if err != nil || p.Changed != 0 || len(p.Issues) != 0 {
		t.Fatalf("no-op %+v %v", p, err)
	}
	r, err := RenameEntries([]string{a}, RenameConfig{Mode: "single", Name: "A.txt"})
	if err != nil || r.Items[0].Error != "" {
		t.Fatalf("case rename %+v %v", r, err)
	}
	if _, err = os.Stat(filepath.Join(root, "A.txt")); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if os.Symlink(b, link) == nil {
		if _, err = PlanRenames([]string{link}, RenameConfig{Mode: "single", Name: "new"}); err == nil {
			t.Fatal("link accepted")
		}
	}
}
