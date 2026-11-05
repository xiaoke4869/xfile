package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtractNestedDestinationAndDuplicatePolicies(t *testing.T) {
	for _, policy := range []string{"suffix", "parentheses", "block"} {
		t.Run(policy, func(t *testing.T) {
			root, _ := Canonical(t.TempDir())
			source := filepath.Join(root, "source")
			target := filepath.Join(source, "output")
			put(t, filepath.Join(source, "1", "照片.jpg"), "one")
			put(t, filepath.Join(source, "2", "deep", "照片.jpg"), "two")
			put(t, filepath.Join(source, ".hidden"), "hidden")
			put(t, filepath.Join(source, "2", "directory.txt"), "directory collision")
			put(t, filepath.Join(target, "照片.jpg"), "existing")
			suffix := "__2"
			if policy == "parentheses" {
				suffix = " (2)"
			}
			put(t, filepath.Join(target, "照片"+suffix+".jpg"), "existing suffix")
			if err := os.MkdirAll(filepath.Join(target, "directory.txt"), 0755); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(root, "outside.txt")
			put(t, outside, "outside")
			linkCount := 1
			if err := os.Symlink(source, filepath.Join(source, "cycle")); err != nil {
				linkCount = 0
				t.Logf("平台不允许创建测试链接：%v", err)
			}
			c := Config{Sources: []Source{{Path: source}}, Target: target, Mode: "extract", Duplicate: policy}
			p, err := BuildPlan(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			if policy == "block" {
				if len(p.Issues) != 3 {
					t.Fatalf("expected 3 conflicts: %+v", p.Issues)
				}
				if b, _ := os.ReadFile(filepath.Join(source, "1", "照片.jpg")); string(b) != "one" {
					t.Fatal("preview moved source")
				}
				return
			}
			if len(p.Issues) > 0 || len(p.Files) != 4 || len(p.Excluded) != linkCount || p.GroupCount != 1 {
				t.Fatalf("bad extraction plan %+v", p)
			}
			names := map[string]bool{}
			for _, f := range p.Files {
				if filepath.Dir(f.Target) != target || Within(target, f.Source) {
					t.Fatalf("not flattened or destination rescanned: %+v", f)
				}
				names[filepath.Base(f.Target)] = true
			}
			for _, name := range []string{"照片__3.jpg", "照片__4.jpg", "directory__2.txt", ".hidden"} {
				if policy == "parentheses" {
					name = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(name, "__2", " (2)"), "__3", " (3)"), "__4", " (4)")
				}
				if !names[name] {
					t.Fatalf("missing %s: %v", name, names)
				}
			}
			task := Run(context.Background(), p, filepath.Join(root, "task"), func(Task) {})
			if task.Status != "succeeded" || task.Moved != 4 || task.Before != 2 || task.After != 6 {
				t.Fatalf("bad task %+v", task)
			}
			if b, _ := os.ReadFile(filepath.Join(target, "照片.jpg")); string(b) != "existing" {
				t.Fatal("overwritten")
			}
			if _, err := os.Stat(filepath.Join(source, "2", "deep")); err != nil {
				t.Fatal("empty source directory removed")
			}
			if b, _ := os.ReadFile(outside); string(b) != "outside" {
				t.Fatal("link followed")
			}
			again, err := BuildPlan(context.Background(), c)
			if err != nil || len(again.Files) != 0 {
				t.Fatalf("output extracted again: %+v %v", again, err)
			}
		})
	}
}

func TestNestedMergeExcludesOutputAndFlatRecovery(t *testing.T) {
	for _, mode := range []string{"merge", "extract"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := Canonical(t.TempDir())
			source := filepath.Join(root, "source")
			target := filepath.Join(source, "output")
			put(t, filepath.Join(source, "one", "a.txt"), "a")
			put(t, filepath.Join(source, "two", "a.txt"), "b")
			c := Config{Sources: []Source{{Path: source}}, Target: target, Mode: mode}
			p, err := BuildPlan(context.Background(), c)
			if err != nil || len(p.Issues) > 0 || len(p.Files) != 2 {
				t.Fatalf("bad plan %+v %v", p, err)
			}
			if err := os.Mkdir(target, 0755); err != nil {
				t.Fatal(err)
			}
			fresh, err := BuildPlan(context.Background(), c)
			if err != nil || fresh.ID != p.ID {
				t.Fatal("empty target creation changed plan", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			dir := filepath.Join(root, "tasks", "first")
			first := Run(ctx, p, dir, func(task Task) {
				if task.Moved == 1 {
					cancel()
				}
			})
			if first.Status != "cancelled" || first.Moved != 1 {
				t.Fatalf("bad interruption %+v", first)
			}
			recovery, err := RecoveryPlan(context.Background(), dir)
			if err != nil || len(recovery.Files) != 1 || len(recovery.Issues) > 0 {
				t.Fatalf("bad recovery %+v %v", recovery, err)
			}
			if err := ValidateRecoveryPlan(context.Background(), recovery); err != nil {
				t.Fatal(err)
			}
			last := Run(context.Background(), recovery, filepath.Join(root, "tasks", "second"), func(Task) {})
			if last.Status != "succeeded" || last.Moved != 1 {
				t.Fatalf("bad recovery task %+v", last)
			}
		})
	}
}

func TestDirectoryTextFormatsAndExclusiveExport(t *testing.T) {
	ctx := context.Background()
	root, _ := Canonical(t.TempDir())
	text, err := DirectoryText(ctx, root)
	if err != nil || text != "" {
		t.Fatalf("empty %q %v", text, err)
	}
	put(t, filepath.Join(root, "1.txt"), "one")
	put(t, filepath.Join(root, "2.txt"), "two")
	text, err = DirectoryText(ctx, root)
	if err != nil || text != "1.txt\n2.txt\n" {
		t.Fatalf("flat %q %v", text, err)
	}
	multi, _ := Canonical(t.TempDir())
	put(t, filepath.Join(multi, "1", "未命名文件夹", "文件名-1.jpg"), "one")
	put(t, filepath.Join(multi, "2", "文件名-21.jpg"), "two")
	put(t, filepath.Join(multi, "2", "文件名-23.jpg"), "three")
	put(t, filepath.Join(multi, "3", "文件名-21.jpg"), "four")
	want := ".\n├── 1\n│   └── 未命名文件夹\n│       └── 文件名-1.jpg\n├── 2\n│   ├── 文件名-21.jpg\n│   └── 文件名-23.jpg\n└── 3\n    └── 文件名-21.jpg\n"
	text, err = DirectoryText(ctx, multi)
	if err != nil || text != want {
		t.Fatalf("tree\ngot: %q\nwant: %q\n%v", text, want, err)
	}
	out := filepath.Join(multi, "结构.txt")
	if err := WriteDirectoryText(ctx, multi, out); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != want {
		t.Fatal("export differs")
	}
	if err := WriteDirectoryText(ctx, root, out); err == nil {
		t.Fatal("overwrite allowed")
	}
	if b, _ := os.ReadFile(out); string(b) != want {
		t.Fatal("existing export overwritten")
	}
	special, _ := Canonical(t.TempDir())
	put(t, filepath.Join(special, ".hidden"), "hidden")
	name, escaped := "line\nname.txt", "line\\nname.txt"
	if runtime.GOOS == "windows" {
		name, escaped = "line-name.txt", "line-name.txt"
	}
	put(t, filepath.Join(special, name), "escaped")
	if err := os.Mkdir(filepath.Join(special, "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	wantSpecial := ".\n├── .hidden\n├── empty\n└── " + escaped + "\n"
	if err := os.Symlink(special, filepath.Join(special, "link")); err == nil {
		wantSpecial = ".\n├── .hidden\n├── empty\n├── " + escaped + "\n└── link [链接]\n"
	} else {
		t.Logf("平台不允许创建测试链接：%v", err)
	}
	text, err = DirectoryText(ctx, special)
	if err != nil || text != wantSpecial {
		t.Fatalf("special names %q %v", text, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := DirectoryText(cancelled, root); err == nil {
		t.Fatal("cancel ignored")
	}
	if _, err := DirectoryText(ctx, filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing directory accepted")
	}
}

func TestMultipleExtractionSourcesAndNestedTargetDepth(t *testing.T) {
	root, _ := Canonical(t.TempDir())
	a, b := filepath.Join(root, "A"), filepath.Join(root, "B")
	put(t, filepath.Join(a, "a", "same.txt"), "a")
	put(t, filepath.Join(b, "b", "same.txt"), "b")
	put(t, filepath.Join(b, "b", "same__2.txt"), "c")
	c := Config{Sources: []Source{{Path: b}, {Path: a}}, Target: filepath.Join(a, "output"), Mode: "extract"}
	p, err := BuildPlan(context.Background(), c)
	if err != nil || len(p.Issues) > 0 || len(p.Files) != 3 {
		t.Fatalf("multiple sources %+v %v", p, err)
	}
	unique := map[string]bool{}
	for _, f := range p.Files {
		if unique[f.Target] {
			t.Fatal("duplicate target")
		}
		unique[f.Target] = true
	}
	task := Run(context.Background(), p, filepath.Join(root, "task"), func(Task) {})
	if task.Status != "succeeded" || task.Moved != 3 {
		t.Fatalf("multiple sources execution %+v", task)
	}
	// A newly selected relative depth must skip a destination before collecting units.
	put(t, filepath.Join(a, "a", "new.txt"), "new")
	c.Sources = []Source{{Path: a, Depth: 1}}
	c.Mode = "merge"
	p, err = BuildPlan(context.Background(), c)
	if err != nil || len(p.Issues) > 0 || len(p.Files) != 1 || len(p.Groups) != 1 {
		t.Fatalf("destination became a depth unit %+v %v", p, err)
	}
	for _, change := range []func(*Config){func(c *Config) { c.Mode = "unknown" }, func(c *Config) { c.Duplicate = "overwrite" }, func(c *Config) { c.Mode = "extract" }} {
		invalid := c
		change(&invalid)
		if _, err := BuildPlan(context.Background(), invalid); err == nil {
			t.Fatalf("invalid config accepted %+v", invalid)
		}
	}
}
