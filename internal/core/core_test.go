package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, p, content string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(content), 0644); e != nil {
		t.Fatal(e)
	}
}
func configFor(t *testing.T) (Config, string) {
	t.Helper()
	root := t.TempDir()
	a := filepath.Join(root, "sources", "A", "1.2 检验资料")
	b := filepath.Join(root, "sources", "B", "deep", "1.2 检验报告")
	put(t, filepath.Join(a, "a.pdf"), "one")
	put(t, filepath.Join(a, "附件", "b.txt"), "two")
	put(t, filepath.Join(b, "a.pdf"), "three")
	c := Config{Sources: []Source{{a, 0}, {b, 0}}, Target: filepath.Join(root, "output"), Prefix: true, Rule: "numeric", Unmatched: "keep", Strict: true}
	return c, root
}
func TestCanonicalHomePaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "/", "/Library/Application Support/apifox", "/资料 目录/../Library/Application Support/apifox"} {
		t.Run("~"+suffix, func(t *testing.T) {
			want, err := Canonical(filepath.Join(home, filepath.FromSlash(suffix)))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Canonical("~" + suffix)
			if err != nil || got != want {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
		})
	}
	for _, path := range []string{"~other/Library", "./~/Library", "Library/~cache"} {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		want, err := Canonical(absolute)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := Canonical(path); err != nil || got != want {
			t.Fatalf("literal path %q: got %q, %v; want %q", path, got, err, want)
		}
	}
}
func TestPrefixRules(t *testing.T) {
	cases := []struct {
		name, rule, sep, pattern string
		n                        int
		expected                 string
		matched                  bool
	}{
		{"1.2 检验资料", "numeric", "", "", 0, "1.2", true}, {"001 病例", "numeric", "", "", 0, "001", true}, {"1.2资料", "numeric", "", "", 0, "1.2资料", false},
		{"AB-001-资料", "separator", "-", "", 0, "AB", true}, {"1.2___资料", "separator", "___", "", 0, "1.2", true}, {"1.2\t 资料", "separator", "whitespace", "", 0, "1.2", true},
		{"中文资料", "length", "", "", 2, "中文", true}, {"👨‍👩‍👧资料", "length", "", "", 1, "👨‍👩‍👧", true}, {"e\u0301资料", "length", "", "", 1, "e\u0301", true},
		{"AB-001 资料", "regex", "", "^([A-Z]+-[0-9]+)", 0, "AB-001", true}, {"资料AB", "regex", "", "AB", 0, "资料AB", false},
	}
	for _, c := range cases {
		t.Run(c.name+c.rule, func(t *testing.T) {
			got, matched, e := Extract(c.name, Config{Prefix: true, Rule: c.rule, Separator: c.sep, Pattern: c.pattern, Length: c.n})
			if e != nil || got != c.expected || matched != c.matched {
				t.Fatalf("got %q %v %v", got, matched, e)
			}
		})
	}
	for _, c := range []Config{{Prefix: true, Rule: "regex", Pattern: "["}, {Prefix: true, Rule: "separator", Separator: "_"}, {Prefix: true, Rule: "length", Length: 0}, {Prefix: true, Rule: "regex", Pattern: "^()"}, {Prefix: true, Rule: "numeric", Unmatched: "block"}} {
		if _, _, e := Extract("_资料", c); e == nil {
			t.Fatal("invalid config accepted", c)
		}
	}
}
func TestPreviewAndExecution(t *testing.T) {
	c, root := configFor(t)
	put(t, filepath.Join(c.Target, "1.2", "a.pdf"), "existing")
	put(t, filepath.Join(c.Target, "1.2", "a__2.pdf"), "existing 2")
	p, e := BuildPlan(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Issues) > 0 || len(p.Files) != 3 || p.GroupCount != 1 {
		t.Fatalf("bad plan %+v", p)
	}
	names := map[string]bool{}
	for _, f := range p.Files {
		names[filepath.Base(f.Target)] = true
		if _, e := os.Stat(f.Source); e != nil {
			t.Fatal("preview modified source")
		}
		if _, e := os.Stat(f.Target); !os.IsNotExist(e) {
			t.Fatal("preview wrote target")
		}
	}
	if !names["a__3.pdf"] || !names["a__4.pdf"] || !names["b.txt"] {
		t.Fatal(names)
	}
	task := Run(context.Background(), p, filepath.Join(root, "tasks", "first"), func(Task) {})
	if task.Status != "succeeded" || task.Moved != 3 || task.Before != 2 || task.After != 5 || task.Remaining != 0 {
		t.Fatalf("bad task %+v", task)
	}
	for _, f := range p.Files {
		if _, e := os.Stat(f.Source); !os.IsNotExist(e) {
			t.Fatal("source still exists")
		}
		sum, e := HashFile(context.Background(), f.Target)
		if e != nil || sum == "" {
			t.Fatal(e)
		}
	}
	old, _ := os.ReadFile(filepath.Join(c.Target, "1.2", "a.pdf"))
	if string(old) != "existing" {
		t.Fatal("existing content overwritten")
	}
	for _, name := range []string{"plan.json", "summary.json", "operations.jsonl", "moves.csv", "renames.csv", "directory-names.csv", "report.md"} {
		if _, e := os.Stat(filepath.Join(task.Directory, name)); e != nil {
			t.Fatal(e)
		}
	}
	b, _ := os.ReadFile(filepath.Join(task.Directory, "summary.json"))
	var saved Task
	if json.Unmarshal(b, &saved) != nil || saved.Moved != 3 {
		t.Fatal("invalid saved report")
	}
}
func TestFullNameAndIndependentDepth(t *testing.T) {
	c, root := configFor(t)
	c.Prefix = false
	c.Sources = []Source{{filepath.Join(root, "sources", "A"), 1}, {filepath.Join(root, "sources", "B"), 2}}
	p, e := BuildPlan(context.Background(), c)
	if e != nil || len(p.Issues) > 0 || len(p.Files) != 3 || p.GroupCount != 2 {
		t.Fatalf("bad depth plan %+v %v", p, e)
	}
	if _, e := os.Stat(c.Target); !os.IsNotExist(e) {
		t.Fatal("preview created destination")
	}
}
func TestUnsafeRangesAndLinks(t *testing.T) {
	c, _ := configFor(t)
	c.Target = c.Sources[0].Path
	if _, e := BuildPlan(context.Background(), c); e == nil {
		t.Fatal("same source and target accepted")
	}
	c.Target = filepath.Dir(c.Sources[0].Path)
	if _, e := BuildPlan(context.Background(), c); e == nil {
		t.Fatal("ancestor target accepted")
	}
	c.Sources = append(c.Sources, Source{filepath.Join(c.Sources[0].Path, "附件"), 0})
	if _, e := Scan(context.Background(), c.Sources); e == nil {
		t.Fatal("overlapping source accepted")
	}
}
func TestConflictAtExecutionNeverOverwrites(t *testing.T) {
	c, root := configFor(t)
	p, e := BuildPlan(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	put(t, p.Files[0].Target, "late arrival")
	task := Run(context.Background(), p, filepath.Join(root, "task"), func(Task) {})
	if task.Status != "failed" {
		t.Fatal(task.Status)
	}
	b, _ := os.ReadFile(p.Files[0].Target)
	if string(b) != "late arrival" {
		t.Fatal("overwrite")
	}
	if _, e := os.Stat(p.Files[0].Source); e != nil {
		t.Fatal("source lost")
	}
}
func TestCopyVerifiedBeforeSourceDeletion(t *testing.T) {
	c, _ := configFor(t)
	p, e := BuildPlan(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	f := p.Files[0]
	if e = makeParents(f.Target); e != nil {
		t.Fatal(e)
	}
	sum, e := HashFile(context.Background(), f.Source)
	if e != nil {
		t.Fatal(e)
	}
	state := FileState{Source: f.Source, Target: f.Target}
	var phases []string
	var copied int64
	e = copyVerified(context.Background(), f, &state, sum, func(s string) error { phases = append(phases, s); return nil }, func(n int64) { copied = n })
	if copied != f.Size {
		t.Fatalf("progress bytes %d, want %d", copied, f.Size)
	}
	if e != nil {
		t.Fatal(e)
	}
	if strings.Join(phases, ",") != "copying,destination_verified,destination_committed" {
		t.Fatal(phases)
	}
	if _, e = os.Stat(f.Source); !os.IsNotExist(e) {
		t.Fatal("source remains")
	}
	actual, e := HashFile(context.Background(), f.Target)
	if e != nil || actual != sum {
		t.Fatal("copy hash mismatch")
	}
}
func TestBadCopyOrJournalKeepsSource(t *testing.T) {
	for _, failJournal := range []bool{false, true} {
		t.Run(strings.ToUpper(strconvBool(failJournal)), func(t *testing.T) {
			c, _ := configFor(t)
			p, _ := BuildPlan(context.Background(), c)
			f := p.Files[0]
			_ = makeParents(f.Target)
			state := FileState{}
			sum, _ := HashFile(context.Background(), f.Source)
			if !failJournal {
				sum = "wrong"
			}
			e := copyVerified(context.Background(), f, &state, sum, func(s string) error {
				if failJournal && s == "destination_committed" {
					return errors.New("disk error")
				}
				return nil
			})
			if e == nil {
				t.Fatal("failure not detected")
			}
			if _, e := os.Stat(f.Source); e != nil {
				t.Fatal("unverified source deleted")
			}
		})
	}
}
func strconvBool(b bool) string {
	if b {
		return "journal"
	}
	return "hash"
}
func TestCancellationKeepsFiles(t *testing.T) {
	c, root := configFor(t)
	p, _ := BuildPlan(context.Background(), c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	task := Run(ctx, p, filepath.Join(root, "task"), func(Task) {})
	if task.Status != "cancelled" || task.Moved != 0 || task.Remaining != 3 {
		t.Fatalf("bad cancellation %+v", task)
	}
}
func TestNameStructureConflict(t *testing.T) {
	c, _ := configFor(t)
	put(t, filepath.Join(c.Target, "1.2"), "file blocks folder")
	p, e := BuildPlan(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Issues) == 0 {
		t.Fatal("structure conflict not detected")
	}
}

func TestRecoveryContinuesOnlyPendingFiles(t *testing.T) {
	c, root := configFor(t)
	p, e := BuildPlan(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	dir := filepath.Join(root, "tasks", "original")
	first := Run(ctx, p, dir, func(task Task) {
		if task.Moved == 1 {
			cancel()
		}
	})
	if first.Status != "cancelled" || first.Moved != 1 {
		t.Fatalf("unexpected interrupted task %+v", first)
	}
	recovered, e := RecoveryPlan(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	if len(recovered.Files) != 2 || len(recovered.Issues) > 0 {
		t.Fatalf("bad recovery plan %+v", recovered)
	}
	if e = ValidateRecoveryPlan(context.Background(), recovered); e != nil {
		t.Fatal(e)
	}
	resumed := Run(context.Background(), recovered, filepath.Join(root, "tasks", "resumed"), func(Task) {})
	if resumed.Status != "succeeded" || resumed.Moved != 2 {
		t.Fatalf("bad resumed task %+v", resumed)
	}
	if e = RecordRecoveryResult(recovered, resumed); e != nil {
		t.Fatal(e)
	}
	_, original, e := InspectRecovery(context.Background(), dir)
	if e != nil || original.Status != "succeeded" || original.Moved != 3 {
		t.Fatalf("original reconciliation %+v %v", original, e)
	}
}
func TestRecoveryNeverDeletesDuplicateSource(t *testing.T) {
	c, root := configFor(t)
	p, _ := BuildPlan(context.Background(), c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := filepath.Join(root, "tasks", "original")
	Run(ctx, p, dir, func(Task) {})
	put(t, p.Files[0].Target, "unrelated late file")
	recovered, e := RecoveryPlan(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	if len(recovered.Issues) == 0 {
		t.Fatal("duplicate requires review")
	}
	if _, e := os.Stat(p.Files[0].Source); e != nil {
		t.Fatal("recovery deleted source")
	}
	b, _ := os.ReadFile(p.Files[0].Target)
	if string(b) != "unrelated late file" {
		t.Fatal("recovery changed target")
	}
}
func TestSourceAddedDuringRunNeedsReview(t *testing.T) {
	c, root := configFor(t)
	p, _ := BuildPlan(context.Background(), c)
	added := false
	task := Run(context.Background(), p, filepath.Join(root, "task"), func(task Task) {
		if !added && task.Moved == 1 {
			added = true
			put(t, filepath.Join(c.Sources[0].Path, "external.txt"), "external")
		}
	})
	if task.Status != "needs_review" {
		t.Fatalf("new source not detected %+v", task)
	}
}
func TestSymlinkExcludedAndTargetReplacementBlocked(t *testing.T) {
	c, root := configFor(t)
	outside := filepath.Join(root, "outside.txt")
	put(t, outside, "do not move")
	if e := os.Symlink(outside, filepath.Join(c.Sources[0].Path, "link.txt")); e != nil {
		t.Skip(e)
	}
	p, e := BuildPlan(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Excluded) != 1 || len(p.Files) != 3 {
		t.Fatal("link not excluded")
	}
	if e = os.MkdirAll(c.Target, 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(filepath.Dir(outside), filepath.Join(c.Target, "1.2")); e != nil {
		t.Fatal(e)
	}
	task := Run(context.Background(), p, filepath.Join(root, "task"), func(Task) {})
	if task.Status != "failed" {
		t.Fatal("symlink replacement not blocked")
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "do not move" {
		t.Fatal("outside content changed")
	}
}
