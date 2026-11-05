package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"
)

type Source struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}
type Config struct {
	Mode      string   `json:"mode,omitempty"`
	Duplicate string   `json:"duplicate,omitempty"`
	Sources   []Source `json:"sources"`
	Target    string   `json:"target"`
	Prefix    bool     `json:"prefix"`
	Rule      string   `json:"rule"`
	Separator string   `json:"separator"`
	Length    int      `json:"length"`
	Pattern   string   `json:"pattern"`
	Unmatched string   `json:"unmatched"`
	Strict    bool     `json:"strict"`
}
type Entry struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Dir      bool   `json:"dir"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Link     bool   `json:"link"`
	Regular  bool   `json:"regular"`
}
type File struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relative string `json:"relative"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Renamed  bool   `json:"renamed"`
	info     fs.FileInfo
}
type Group struct {
	Source   string `json:"source"`
	Original string `json:"original"`
	Name     string `json:"name"`
	Matched  bool   `json:"matched"`
	Count    int    `json:"count"`
}
type Plan struct {
	RecoveryFrom string    `json:"recoveryFrom,omitempty"`
	ID           string    `json:"id"`
	Revision     int       `json:"revision"`
	Created      time.Time `json:"created"`
	Config       Config    `json:"config"`
	Files        []File    `json:"files"`
	Groups       []Group   `json:"groups"`
	Existing     []Entry   `json:"existing"`
	Issues       []string  `json:"issues"`
	Excluded     []string  `json:"excluded"`
	Bytes        int64     `json:"bytes"`
	GroupCount   int       `json:"groupCount"`
}
type unit struct {
	path  string
	files []File
}
type Snapshot struct {
	units    []unit
	Excluded []string
	Sources  []Source
}

func CheckContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Canonical expands the current user's home and resolves existing ancestors too.
func Canonical(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("请选择目录")
	}
	if path == "~" || len(path) > 1 && path[0] == '~' && os.IsPathSeparator(path[1]) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var tail []string
	current := filepath.Clean(abs)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}
func Within(parent, child string) bool {
	r, e := filepath.Rel(parent, child)
	return e == nil && (r == "." || r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)))
}
func PathsOverlap(a, b string) bool {
	ka, kb := ProbeNameKey(a), ProbeNameKey(b)
	return Within(ka(a), ka(b)) || Within(kb(b), kb(a))
}
func ValidateTarget(sources []Source, target string) (string, error) {
	p, e := Canonical(target)
	if e != nil {
		return "", e
	}
	for _, s := range sources {
		if key := ProbeNameKey(p); Within(key(p), key(s.Path)) {
			return "", errors.New("目标不能与来源相同，也不能是来源的上级目录")
		}
	}
	return p, nil
}
func existingDirectory(path string) (string, error) {
	for {
		info, e := os.Stat(path)
		if e == nil && info.IsDir() {
			return path, nil
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		next := filepath.Dir(path)
		if next == path {
			return "", fmt.Errorf("找不到可访问的上级目录：%s", path)
		}
		path = next
	}
}
func CheckSpace(p *Plan) error {
	volume, free, e := DiskInfo(p.Config.Target)
	if e != nil {
		return fmt.Errorf("无法检查目标磁盘空间：%w", e)
	}
	var need uint64
	volumes := map[string]string{}
	for _, f := range p.Files {
		dir := filepath.Dir(f.Source)
		sourceVolume, ok := volumes[dir]
		if !ok {
			sourceVolume, _, e = DiskInfo(dir)
			if e != nil {
				return e
			}
			volumes[dir] = sourceVolume
		}
		if sourceVolume != volume {
			need += uint64(f.Size)
		}
	}
	if need > free {
		return fmt.Errorf("目标磁盘空间不足：跨磁盘待复制 %d 字节，可用 %d 字节", need, free)
	}
	return nil
}
func ValidateSources(sources []Source) ([]Source, error) {
	if len(sources) == 0 {
		return nil, errors.New("至少选择一个来源目录")
	}
	result := make([]Source, 0, len(sources))
	for _, s := range sources {
		if s.Depth < 0 || s.Depth > 128 {
			return nil, errors.New("查找深度应为 0–128；0 表示直接归并选中目录")
		}
		p, e := Canonical(s.Path)
		if e != nil {
			return nil, e
		}
		i, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if !i.IsDir() {
			return nil, fmt.Errorf("来源不是目录：%s", p)
		}
		for _, r := range result {
			if PathsOverlap(r.Path, p) {
				return nil, fmt.Errorf("来源范围重复或重叠：%s 与 %s", r.Path, p)
			}
		}
		result = append(result, Source{p, s.Depth})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

func Scan(ctx context.Context, sources []Source, exclude ...string) (*Snapshot, error) {
	sources, e := ValidateSources(sources)
	if e != nil {
		return nil, e
	}
	// A destination inside a source is never a source, including during final verification.
	type excludedPath struct {
		path string
		key  func(string) string
	}
	var excluded []excludedPath
	for _, path := range exclude {
		p, err := Canonical(path)
		if err != nil {
			return nil, err
		}
		key := ProbeNameKey(p)
		excluded = append(excluded, excludedPath{key(p), key})
	}
	isExcluded := func(path string) bool {
		for _, x := range excluded {
			if Within(x.path, x.key(path)) {
				return true
			}
		}
		return false
	}
	snap := &Snapshot{Sources: sources, Excluded: []string{}}
	for _, source := range sources {
		var find func(string, int) error
		find = func(path string, depth int) error {
			if isExcluded(path) {
				return nil
			}
			if e := CheckContext(ctx); e != nil {
				return e
			}
			if depth == source.Depth {
				u := unit{path: path, files: []File{}}
				e := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if e := CheckContext(ctx); e != nil {
						return e
					}
					if isExcluded(p) {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if d.Type()&os.ModeSymlink != 0 {
						snap.Excluded = append(snap.Excluded, p+"（链接）")
						return nil
					}
					if d.IsDir() {
						return nil
					}
					info, err := d.Info()
					if err != nil {
						return err
					}
					if !info.Mode().IsRegular() {
						snap.Excluded = append(snap.Excluded, p+"（特殊文件）")
						return nil
					}
					rel, err := filepath.Rel(path, p)
					if err != nil {
						return err
					}
					u.files = append(u.files, File{Source: p, Relative: rel, Size: info.Size(), Modified: info.ModTime().UnixNano(), info: info})
					return nil
				})
				if e != nil {
					return e
				}
				snap.units = append(snap.units, u)
				return nil
			}
			entries, e := os.ReadDir(path)
			if e != nil {
				return e
			}
			for _, d := range entries {
				p := filepath.Join(path, d.Name())
				if d.Type()&os.ModeSymlink != 0 {
					snap.Excluded = append(snap.Excluded, p+"（链接）")
					continue
				}
				if d.IsDir() {
					if e := find(p, depth+1); e != nil {
						return e
					}
				} else {
					snap.Excluded = append(snap.Excluded, p+"（位于所选深度以上）")
				}
			}
			return nil
		}
		if e := find(source.Path, 0); e != nil {
			return nil, e
		}
	}
	return snap, nil
}

var numeric = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)*)(?:\s|$)`)

func Extract(name string, c Config) (string, bool, error) {
	if !c.Prefix {
		return name, true, nil
	}
	input := strings.TrimSpace(name)
	out := ""
	matched := true
	switch c.Rule {
	case "numeric":
		m := numeric.FindStringSubmatch(input)
		if len(m) > 1 {
			out = m[1]
		} else {
			matched = false
		}
	case "separator":
		if c.Separator == "" {
			return "", false, errors.New("分隔符不能为空")
		}
		index := -1
		if c.Separator == "whitespace" {
			index = strings.IndexFunc(input, unicode.IsSpace)
		} else {
			index = strings.Index(input, c.Separator)
		}
		if index < 0 {
			matched = false
		} else {
			out = input[:index]
		}
	case "length":
		if c.Length < 1 || c.Length > 255 {
			return "", false, errors.New("字符数量应为 1–255")
		}
		g := uniseg.NewGraphemes(input)
		n := 0
		for g.Next() && n < c.Length {
			out += g.Str()
			n++
		}
	case "regex":
		if c.Pattern == "" {
			return "", false, errors.New("请输入正则表达式")
		}
		re, err := regexp.Compile(c.Pattern)
		if err != nil {
			return "", false, fmt.Errorf("正则表达式无效：%w", err)
		}
		idx := re.FindStringSubmatchIndex(input)
		if idx == nil || idx[0] != 0 {
			matched = false
		} else if len(idx) > 2 {
			if idx[2] < 0 {
				return "", false, errors.New("第一个捕获组未参与匹配")
			}
			out = input[idx[2]:idx[3]]
		} else {
			out = input[idx[0]:idx[1]]
		}
	default:
		return "", false, errors.New("请选择有效的前缀提取方式")
	}
	if !matched {
		if c.Unmatched == "block" {
			return "", false, fmt.Errorf("目录名未匹配规则：%s", name)
		}
		return name, false, nil
	}
	out = strings.TrimSpace(out)
	if err := ValidName(out); err != nil {
		return "", false, err
	}
	return out, true, nil
}
func ValidName(s string) error {
	if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00<>:\"|?*") || strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
		return fmt.Errorf("无法作为跨平台目录或文件名：%q", s)
	}
	for _, r := range s {
		if r < 32 {
			return fmt.Errorf("名称含控制字符：%q", s)
		}
	}
	base := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || regexp.MustCompile(`^(COM|LPT)[1-9]$`).MatchString(base) {
		return fmt.Errorf("Windows 保留名称：%s", s)
	}
	return nil
}
func TargetEntries(ctx context.Context, target string) ([]Entry, error) {
	result := []Entry{}
	if _, e := os.Stat(target); os.IsNotExist(e) {
		return result, nil
	}
	e := filepath.WalkDir(target, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if e = CheckContext(ctx); e != nil {
			return e
		}
		if p == target {
			return nil
		}
		i, e := d.Info()
		if e != nil {
			return e
		}
		result = append(result, Entry{Path: p, Name: d.Name(), Dir: d.IsDir(), Size: i.Size(), Modified: i.ModTime().UnixNano(), Link: d.Type()&os.ModeSymlink != 0, Regular: i.Mode().IsRegular()})
		return nil
	})
	return result, e
}

// ProbeNameKey is read-only. Existing ancestors determine case behaviour; Unicode
// normalization is conservative so the same plan is also safe on common macOS volumes.
func ProbeNameKey(target string) func(string) string {
	sensitive := CaseSensitive(target)
	return func(p string) string {
		p = norm.NFC.String(filepath.Clean(p))
		if !sensitive {
			p = strings.ToLower(p)
		}
		return p
	}
}
func MakePlan(ctx context.Context, snap *Snapshot, c Config, existing []Entry) (*Plan, error) {
	target, e := ValidateTarget(snap.Sources, c.Target)
	if e != nil {
		return nil, e
	}
	c.Target = target
	c.Sources = snap.Sources
	if c.Mode != "" && c.Mode != "merge" && c.Mode != "extract" {
		return nil, errors.New("无效的文件操作模式")
	}
	if c.Duplicate != "" && c.Duplicate != "suffix" && c.Duplicate != "parentheses" && c.Duplicate != "block" {
		return nil, errors.New("无效的重名处理规则")
	}
	if c.Mode == "extract" {
		for _, source := range c.Sources {
			if source.Depth != 0 {
				return nil, errors.New("递归提取的来源深度必须为 0")
			}
		}
	}
	p := &Plan{Created: time.Now(), Config: c, Files: []File{}, Groups: []Group{}, Existing: existing, Issues: []string{}, Excluded: append([]string{}, snap.Excluded...)}
	if i, e := os.Stat(target); e == nil && !i.IsDir() {
		return nil, errors.New("目标路径已被文件占用")
	}
	key := ProbeNameKey(target)
	type reserved struct {
		dir  bool
		path string
	}
	occupied := map[string]reserved{}
	occupied[key(target)] = reserved{true, target}
	for _, entry := range existing {
		occupied[key(entry.Path)] = reserved{entry.Dir && !entry.Link, entry.Path}
	}
	var ensureDir func(string) (string, error)
	ensureDir = func(path string) (string, error) {
		k := key(path)
		if found, ok := occupied[k]; ok {
			if !found.dir {
				return "", fmt.Errorf("文件或链接占用目标目录：%s", path)
			}
			return found.path, nil
		}
		parent, e := ensureDir(filepath.Dir(path))
		if e != nil {
			return "", e
		}
		path = filepath.Join(parent, filepath.Base(path))
		occupied[k] = reserved{true, path}
		return path, nil
	}
	groups := map[string]bool{}
	for _, u := range snap.units {
		if e := CheckContext(ctx); e != nil {
			return nil, e
		}
		groupPath := target
		if c.Mode == "extract" {
			p.Groups = append(p.Groups, Group{u.path, filepath.Base(u.path), "", true, len(u.files)})
		} else {
			name, matched, err := Extract(filepath.Base(u.path), c)
			if err != nil {
				p.Issues = append(p.Issues, err.Error())
				continue
			}
			if err = ValidName(name); err != nil {
				p.Issues = append(p.Issues, err.Error())
				continue
			}
			group := Group{u.path, filepath.Base(u.path), name, matched, len(u.files)}
			groupPath, err = ensureDir(filepath.Join(target, name))
			if err != nil {
				p.Issues = append(p.Issues, err.Error())
				p.Groups = append(p.Groups, group)
				continue
			}
			group.Name = filepath.Base(groupPath)
			p.Groups = append(p.Groups, group)
		}
		groups[key(groupPath)] = true
		for _, f := range u.files {
			if e := CheckContext(ctx); e != nil {
				return nil, e
			}
			relative := f.Relative
			if c.Mode == "extract" {
				relative = filepath.Base(f.Source)
			}
			valid := true
			for _, part := range strings.Split(relative, string(filepath.Separator)) {
				if err := ValidName(part); err != nil {
					p.Issues = append(p.Issues, err.Error())
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			dir, err := ensureDir(filepath.Join(groupPath, filepath.Dir(relative)))
			if err != nil {
				p.Issues = append(p.Issues, err.Error())
				continue
			}
			name := filepath.Base(relative)
			candidate := filepath.Join(dir, name)
			if r, ok := occupied[key(candidate)]; ok && r.dir && c.Mode != "extract" {
				p.Issues = append(p.Issues, "目录占用文件位置："+candidate)
				continue
			}
			if _, exists := occupied[key(candidate)]; exists && c.Mode == "extract" && c.Duplicate == "block" {
				p.Issues = append(p.Issues, "重名文件阻止执行："+candidate)
				continue
			}
			ext := filepath.Ext(name)
			stem := strings.TrimSuffix(name, ext)
			if stem == "" {
				stem = name
				ext = ""
			}
			for n := 2; ; n++ {
				if _, ok := occupied[key(candidate)]; !ok {
					break
				}
				if c.Mode == "extract" && c.Duplicate == "parentheses" {
					candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, n, ext))
				} else {
					candidate = filepath.Join(dir, fmt.Sprintf("%s__%d%s", stem, n, ext))
				}
			}
			f.Target = candidate
			f.Renamed = filepath.Base(candidate) != name
			occupied[key(candidate)] = reserved{false, candidate}
			p.Files = append(p.Files, f)
			p.Bytes += f.Size
		}
	}
	p.GroupCount = len(groups)
	if len(snap.units) == 0 {
		p.Issues = append(p.Issues, "所选深度没有目录，请调整深度或直接选择归并对象")
	}
	if len(p.Files) == 0 {
		p.Issues = append(p.Issues, "没有可移动的普通文件")
	}
	if err := CheckSpace(p); err != nil {
		p.Issues = append(p.Issues, err.Error())
	}
	p.ID = Fingerprint(p)
	return p, nil
}
func Fingerprint(p *Plan) string {
	b, _ := json.Marshal(struct {
		C Config
		F []File
		G []Group
		E []Entry
		I []string
		X []string
	}{p.Config, p.Files, p.Groups, p.Existing, p.Issues, p.Excluded})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func BuildPlan(ctx context.Context, c Config) (*Plan, error) {
	sources, e := ValidateSources(c.Sources)
	if e != nil {
		return nil, e
	}
	c.Sources = sources
	target, e := ValidateTarget(sources, c.Target)
	if e != nil {
		return nil, e
	}
	c.Target = target
	s, e := Scan(ctx, c.Sources, target)
	if e != nil {
		return nil, e
	}
	entries, e := TargetEntries(ctx, target)
	if e != nil {
		return nil, e
	}
	return MakePlan(ctx, s, c, entries)
}
