package core

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileState struct {
	Source string `json:"source"`
	Target string `json:"target"`
	SHA256 string `json:"sha256"`
	Temp   string `json:"temp"`
	State  string `json:"state"`
	Error  string `json:"error"`
}
type Task struct {
	ID        string          `json:"id"`
	Operation string          `json:"operation,omitempty"`
	Items     []OperationItem `json:"items,omitempty"`
	Status    string          `json:"status"`
	Phase     string          `json:"phase"`
	Current   string          `json:"current"`
	Error     string          `json:"error"`
	Total     int             `json:"total"`
	Moved     int             `json:"moved"`
	Remaining int             `json:"remaining"`
	Retained  int             `json:"retained"`
	Unknown   int             `json:"unknown"`
	Before    int             `json:"before"`
	After     int             `json:"after"`
	Bytes     int64           `json:"bytes"`
	DoneBytes int64           `json:"doneBytes"`
	CopyBytes int64           `json:"copyBytes"`
	Started   time.Time       `json:"started"`
	Finished  time.Time       `json:"finished"`
	Directory string          `json:"directory"`
	States    []FileState     `json:"states"`
}

func AtomicJSON(path string, value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".xfile-state-")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
func HashFile(ctx context.Context, path string) (string, error) {
	if e := CheckParents(path); e != nil {
		return "", e
	}
	info, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("不是普通文件：%s", path)
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil {
		return "", e
	}
	if !os.SameFile(info, opened) {
		return "", errors.New("文件在读取前被替换")
	}
	h := sha256.New()
	buf := make([]byte, 256*1024)
	for {
		if e := CheckContext(ctx); e != nil {
			return "", e
		}
		n, e := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
	}
	after, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	if !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return "", errors.New("文件读取期间发生变化，请关闭写入文件的程序后重试")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func CheckParents(path string) error {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		i, e := os.Lstat(p)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil && i.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("操作路径含链接，已停止：%s", p)
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
func makeParents(path string) error {
	if e := CheckParents(filepath.Dir(path)); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return CheckParents(filepath.Dir(path))
}
func checkSource(f File) error {
	if e := CheckParents(f.Source); e != nil {
		return e
	}
	i, e := os.Lstat(f.Source)
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() || i.Size() != f.Size || i.ModTime().UnixNano() != f.Modified {
		return fmt.Errorf("源文件已变化：%s", f.Source)
	}
	if f.info != nil && !os.SameFile(i, f.info) {
		return fmt.Errorf("源文件已被替换：%s", f.Source)
	}
	return nil
}
func regularCount(entries []Entry) int {
	n := 0
	for _, e := range entries {
		if e.Regular && !e.Link {
			n++
		}
	}
	return n
}

func Run(ctx context.Context, p *Plan, dir string, update func(Task)) (task Task) {
	task = Task{ID: filepath.Base(dir), Status: "running", Phase: "准备", Total: len(p.Files), Bytes: p.Bytes, Before: regularCount(p.Existing), Started: time.Now(), Directory: dir, States: make([]FileState, len(p.Files))}
	for i, f := range p.Files {
		task.States[i] = FileState{Source: f.Source, Target: f.Target, State: "planned"}
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		task.Status = "failed"
		task.Error = e.Error()
		update(task)
		return
	}
	journal, e := os.OpenFile(filepath.Join(dir, "operations.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		task.Status = "failed"
		task.Error = e.Error()
		update(task)
		return
	}
	defer journal.Close()
	lastCheckpoint := time.Time{}
	persist := func(index int, state string) error {
		if index >= 0 {
			task.States[index].State = state
		}
		event := struct {
			Time  time.Time  `json:"time"`
			Index int        `json:"index"`
			Phase string     `json:"phase"`
			File  *FileState `json:"file,omitempty"`
		}{Time: time.Now(), Index: index, Phase: state}
		if index >= 0 {
			copy := task.States[index]
			event.File = &copy
		}
		if e := json.NewEncoder(journal).Encode(event); e != nil {
			return e
		}
		if e := journal.Sync(); e != nil {
			return e
		}
		// The synced journal is authoritative between summary checkpoints.
		if index < 0 || time.Since(lastCheckpoint) >= 2*time.Second {
			if e := AtomicJSON(filepath.Join(dir, "summary.json"), task); e != nil {
				return e
			}
			lastCheckpoint = time.Now()
		}
		update(task)
		return nil
	}
	if e = AtomicJSON(filepath.Join(dir, "plan.json"), p); e != nil {
		task.Status = "failed"
		task.Error = e.Error()
		update(task)
		return
	}
	if e = persist(-1, "started"); e != nil {
		task.Status = "failed"
		task.Error = e.Error()
		update(task)
		return
	}
	for index, f := range p.Files {
		task.CopyBytes = 0
		task.Current = f.Source
		task.Phase = "核对源文件"
		update(task)
		e = CheckContext(ctx)
		if e != nil {
			task.Status = "cancelled"
			break
		}
		if e = checkSource(f); e != nil {
			task.States[index].Error = e.Error()
			task.Status = "failed"
			break
		}
		task.Phase = "读取源文件校验值"
		update(task)
		sum, err := HashFile(ctx, f.Source)
		if err != nil {
			e = err
			task.States[index].Error = e.Error()
			task.Status = "failed"
			break
		}
		task.States[index].SHA256 = sum
		if e = persist(index, "intent_logged"); e != nil {
			task.Status = "failed"
			break
		}
		if e = makeParents(f.Target); e != nil {
			task.States[index].Error = e.Error()
			task.Status = "failed"
			break
		}
		if e = checkSource(f); e != nil {
			task.States[index].Error = e.Error()
			task.Status = "failed"
			break
		}
		task.Phase = "移动文件"
		update(task)
		e = MoveExclusive(f.Source, f.Target)
		if e != nil && CrossDevice(e) {
			task.Phase = "跨磁盘复制与校验"
			update(task)
			e = copyVerified(ctx, f, &task.States[index], sum, func(state string) error { return persist(index, state) }, func(bytes int64) { task.CopyBytes = bytes; update(task) })
		} else if e == nil {
			if err := persist(index, "destination_committed"); err != nil {
				e = err
			}
			if e == nil {
				actual, err := HashFile(context.Background(), f.Target)
				if err != nil {
					e = err
				} else if actual != sum {
					e = errors.New("移动后内容校验不一致，文件保留在目标位置，请人工核对")
				}
			}
		}
		if e != nil {
			task.States[index].Error = e.Error()
			task.Status = "failed"
			break
		}
		if _, err := os.Lstat(f.Source); !os.IsNotExist(err) {
			e = errors.New("源路径仍存在或无法检查，尚未计为移动成功")
			task.States[index].Error = e.Error()
			task.Status = "needs_review"
			break
		}
		task.Moved++
		task.CopyBytes = 0
		task.DoneBytes += f.Size
		if e = persist(index, "completed"); e != nil {
			task.Status = "failed"
			break
		}
	}
	if errors.Is(e, context.Canceled) {
		task.Status = "cancelled"
	}
	if e != nil {
		task.Error = e.Error()
	}
	task.Phase = "结束核对"
	update(task)
	entries, scanErr := TargetEntries(context.Background(), p.Config.Target)
	internalTemps := map[string]bool{}
	for _, state := range task.States {
		if state.Temp != "" {
			internalTemps[state.Temp] = true
		}
	}
	visibleEntries := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if !internalTemps[entry.Path] {
			visibleEntries = append(visibleEntries, entry)
		}
	}
	entries = visibleEntries
	if scanErr != nil {
		task.Status = "needs_review"
		task.Error += "；目标复扫失败：" + scanErr.Error()
	} else {
		task.After = regularCount(entries)
	}
	for i, f := range p.Files {
		source, se := os.Lstat(f.Source)
		target, te := os.Lstat(f.Target)
		if se == nil && source.Mode().IsRegular() {
			task.Remaining++
			if te == nil {
				task.Retained++
			}
		}
		if os.IsNotExist(se) && te == nil && target.Mode().IsRegular() && task.States[i].State == "completed" {
			actual, err := HashFile(context.Background(), f.Target)
			if err == nil && actual == task.States[i].SHA256 {
				continue
			}
			task.Unknown++
			task.States[i].State = "needs_review"
			task.States[i].Error = "结束核对发现目标无法读取或内容变化"
			task.Status = "needs_review"
			task.Error = "已移动文件在结束核对时发生变化"
			continue
		}
		if se != nil && !os.IsNotExist(se) || se == nil && !source.Mode().IsRegular() || os.IsNotExist(se) && (te != nil || task.States[i].State != "completed") {
			task.Unknown++
		}
	}
	if finalSource, err := Scan(context.Background(), p.Config.Sources, p.Config.Target); err != nil {
		task.Status = "needs_review"
		task.Error += "；来源复扫失败：" + err.Error()
	} else {
		known := map[string]bool{}
		for _, f := range p.Files {
			known[f.Source] = true
		}
		extra := 0
		for _, unit := range finalSource.units {
			for _, f := range unit.files {
				if !known[f.Source] {
					extra++
				}
			}
		}
		if extra > 0 {
			task.Status = "needs_review"
			task.Error += fmt.Sprintf("；来源新增 %d 个任务外文件，已保留", extra)
		}
	}
	if task.Status == "running" {
		if task.Moved == task.Total && task.Remaining == 0 && task.Unknown == 0 && task.After-task.Before == task.Total && targetBaselineIntact(p, entries) {
			task.Status = "succeeded"
		} else {
			task.Status = "needs_review"
			task.Error = "数量或目标基线核对存在差异，请查看报告"
		}
	}
	task.Phase = "已结束"
	task.Finished = time.Now()
	task.Current = ""
	if err := persist(-1, task.Status); err != nil {
		task.Status = "needs_review"
		task.Error += "；保存任务报告失败：" + err.Error()
		update(task)
	}
	if err := ExportReports(dir, p, task); err != nil {
		task.Status = "needs_review"
		task.Error += "；导出日志失败：" + err.Error()
		_ = AtomicJSON(filepath.Join(dir, "summary.json"), task)
		update(task)
	}
	return
}

func targetBaselineIntact(p *Plan, entries []Entry) bool {
	current := map[string]Entry{}
	for _, e := range entries {
		current[e.Path] = e
	}
	for _, old := range p.Existing {
		now, ok := current[old.Path]
		if !ok || old.Dir != now.Dir || old.Link != now.Link {
			return false
		}
		if !old.Dir && (old.Size != now.Size || old.Modified != now.Modified) {
			return false
		}
	}
	return true
}

func copyVerified(ctx context.Context, f File, state *FileState, expected string, persist func(string) error, progress ...func(int64)) error {
	src, e := os.Open(f.Source)
	if e != nil {
		return e
	}
	defer src.Close()
	opened, e := src.Stat()
	if e != nil {
		return e
	}
	current, e := os.Lstat(f.Source)
	if e != nil || !os.SameFile(opened, current) {
		return errors.New("复制前源文件被替换")
	}
	out, e := os.CreateTemp(filepath.Dir(f.Target), ".xfile-copy-*")
	if e != nil {
		return e
	}
	state.Temp = out.Name()
	defer out.Close()
	if e = persist("copying"); e != nil {
		return e
	}
	buf := make([]byte, 256*1024)
	var copied int64
	lastProgress := time.Now()
	for {
		if e = CheckContext(ctx); e != nil {
			return e
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, e = out.Write(buf[:n]); e != nil {
				return e
			}
			copied += int64(n)
			if len(progress) > 0 && time.Since(lastProgress) >= 150*time.Millisecond {
				progress[0](copied)
				lastProgress = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if len(progress) > 0 {
		progress[0](copied)
	}
	if e = out.Sync(); e != nil {
		return e
	}
	if e = out.Close(); e != nil {
		return e
	}
	actual, e := HashFile(ctx, state.Temp)
	if e != nil {
		return e
	}
	if actual != expected {
		return errors.New("跨磁盘副本校验失败，源文件已保留")
	}
	if e = persist("destination_verified"); e != nil {
		return e
	}
	if e = checkSource(f); e != nil {
		return e
	}
	sourceHash, e := HashFile(ctx, f.Source)
	if e != nil {
		return e
	}
	if sourceHash != expected {
		return errors.New("源内容发生变化，源文件已保留")
	}
	if e = os.Chtimes(state.Temp, time.Now(), time.Unix(0, f.Modified)); e != nil {
		return e
	}
	if e = CheckParents(f.Target); e != nil {
		return e
	}
	if e = MoveExclusive(state.Temp, f.Target); e != nil {
		return e
	}
	state.Temp = ""
	if e = persist("destination_committed"); e != nil {
		return e
	}
	// No cancellation between verified destination commit and source deletion.
	if e = checkSource(f); e != nil {
		return e
	}
	latest, e := os.Lstat(f.Source)
	if e != nil {
		return e
	}
	if !os.SameFile(opened, latest) {
		return errors.New("源路径被替换，已复制但保留源文件")
	}
	if e = src.Close(); e != nil {
		return e
	}
	return os.Remove(f.Source)
}

func csvText(s string) string {
	trim := strings.TrimLeft(s, " \t\r\n")
	if trim != "" && strings.ContainsAny(trim[:1], "=+-@") {
		return "'" + s
	}
	return s
}
func writeCSV(path string, rows [][]string) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	if _, e = f.WriteString("\xef\xbb\xbf"); e != nil {
		return e
	}
	w := csv.NewWriter(f)
	for _, row := range rows {
		clean := make([]string, len(row))
		for i, s := range row {
			clean[i] = csvText(s)
		}
		if e = w.Write(clean); e != nil {
			return e
		}
	}
	w.Flush()
	if e = w.Error(); e != nil {
		return e
	}
	return f.Sync()
}
func ExportReports(dir string, p *Plan, t Task) error {
	moves := [][]string{{"来源路径", "最终目标路径", "状态", "大小", "SHA256", "错误"}}
	renames := [][]string{{"来源路径", "原文件名", "最终文件名", "目标路径", "状态"}}
	for i, f := range p.Files {
		s := t.States[i]
		moves = append(moves, []string{f.Source, f.Target, s.State, strconv.FormatInt(f.Size, 10), s.SHA256, s.Error})
		if f.Renamed {
			renames = append(renames, []string{f.Source, filepath.Base(f.Source), filepath.Base(f.Target), f.Target, s.State})
		}
	}
	groups := [][]string{{"来源目录", "原目录名", "目标分组名", "匹配规则", "匹配结果"}}
	for _, g := range p.Groups {
		rule := p.Config.Rule
		if !p.Config.Prefix {
			rule = "完整名称"
		}
		name := g.Name
		if p.Config.Mode == "extract" {
			rule = "递归提取"
			name = "."
		}
		groups = append(groups, []string{g.Source, g.Original, name, rule, strconv.FormatBool(g.Matched)})
	}
	for name, rows := range map[string][][]string{"moves.csv": moves, "renames.csv": renames, "directory-names.csv": groups} {
		if e := writeCSV(filepath.Join(dir, name), rows); e != nil {
			return e
		}
	}
	report := fmt.Sprintf("# XFile 移动报告\n\n任务：%s\n\n状态：%s\n\n选中文件：%d\n\n已移动并验证：%d\n\n来源剩余：%d\n\n双份保留：%d\n\n状态待核对：%d\n\n目标执行前：%d\n\n目标执行后：%d\n\n目标净增：%d\n\n排除项：%d\n\n错误：%s\n\n每个文件的原路径、目标路径和校验值见 moves.csv；目录映射见 directory-names.csv；重名文件见 renames.csv。\n", t.ID, t.Status, t.Total, t.Moved, t.Remaining, t.Retained, t.Unknown, t.Before, t.After, t.After-t.Before, len(p.Excluded), t.Error)
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(report), 0600)
}
