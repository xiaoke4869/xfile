package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// InspectRecovery reconciles the journal with actual files. It never deletes a
// source or a temporary copy, including when both source and destination exist.
func InspectRecovery(ctx context.Context, dir string) (*Plan, Task, error) {
	var p Plan
	var t Task
	b, e := os.ReadFile(filepath.Join(dir, "plan.json"))
	if e != nil {
		return nil, t, e
	}
	if e = json.Unmarshal(b, &p); e != nil {
		return nil, t, e
	}
	b, e = os.ReadFile(filepath.Join(dir, "summary.json"))
	if e != nil {
		return nil, t, e
	}
	if e = json.Unmarshal(b, &t); e != nil {
		return nil, t, e
	}
	if len(t.States) != len(p.Files) {
		return nil, t, errors.New("任务文件清单与状态不一致，保留原始日志以供人工核对")
	}
	journal, e := os.Open(filepath.Join(dir, "operations.jsonl"))
	if e != nil {
		return nil, t, e
	}
	defer journal.Close()
	reader := bufio.NewReader(journal)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, t, err
		}
		var event struct {
			Index int        `json:"index"`
			File  *FileState `json:"file"`
		}
		if e = json.Unmarshal(line, &event); e != nil {
			return nil, t, errors.New("操作日志损坏，已停止自动恢复")
		}
		if event.Index < 0 || event.File == nil {
			continue
		}
		if event.Index >= len(t.States) {
			return nil, t, errors.New("操作日志索引错误")
		}
		expected := p.Files[event.Index]
		if event.File.Source != expected.Source || event.File.Target != expected.Target {
			return nil, t, errors.New("操作日志路径与原计划不一致")
		}
		t.States[event.Index] = *event.File
	}
	t.Moved = 0
	t.Remaining = 0
	t.Retained = 0
	t.Unknown = 0
	t.DoneBytes = 0
	t.Current = ""
	t.Error = ""
	t.Status = "needs_review"
	t.Phase = "中断任务核对完成"
	for i, f := range p.Files {
		if e = CheckContext(ctx); e != nil {
			return nil, t, e
		}
		state := &t.States[i]
		source, se := os.Lstat(f.Source)
		target, te := os.Lstat(f.Target)
		srcExists := se == nil && source.Mode().IsRegular()
		dstExists := te == nil && target.Mode().IsRegular()
		switch {
		case os.IsNotExist(se) && dstExists && state.SHA256 != "":
			actual, err := HashFile(ctx, f.Target)
			if err == nil && actual == state.SHA256 {
				state.State = "completed"
				state.Error = ""
				t.Moved++
				t.DoneBytes += f.Size
			} else {
				state.State = "needs_review"
				state.Error = "目标内容无法确认"
				t.Unknown++
			}
		case srcExists && os.IsNotExist(te):
			t.Remaining++
			state.State = "planned"
			state.Error = ""
			if source.Size() != f.Size || source.ModTime().UnixNano() != f.Modified {
				state.State = "needs_review"
				state.Error = "源文件已变化"
				t.Unknown++
			}
			if state.SHA256 != "" {
				sum, err := HashFile(ctx, f.Source)
				if err != nil || sum != state.SHA256 {
					state.State = "needs_review"
					state.Error = "源文件与中断前校验值不符"
					t.Unknown++
				}
			}
			if state.Temp != "" {
				if _, err := os.Lstat(state.Temp); err == nil {
					state.State = "needs_review"
					state.Error = "存在未提交的复制临时文件，请核对后手动清理"
					t.Unknown++
				}
			}
		case srcExists && dstExists:
			t.Remaining++
			t.Retained++
			state.State = "source_retained"
			state.Error = "源与目标均存在，保留两份文件，需人工核对"
		default:
			if srcExists {
				t.Remaining++
			}
			state.State = "needs_review"
			state.Error = "源或目标状态无法确认"
			t.Unknown++
		}
	}
	entries, e := TargetEntries(ctx, p.Config.Target)
	if e != nil {
		return nil, t, e
	}
	temps := map[string]bool{}
	for _, state := range t.States {
		if state.Temp != "" {
			temps[state.Temp] = true
		}
	}
	visible := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if !temps[entry.Path] {
			visible = append(visible, entry)
		}
	}
	entries = visible
	t.After = regularCount(entries)
	if t.Moved == t.Total && t.Remaining == 0 && t.Unknown == 0 {
		t.Status = "succeeded"
		t.Finished = time.Now()
	}
	if t.Unknown > 0 || t.Retained > 0 {
		t.Error = fmt.Sprintf("待核对 %d 项，双份保留 %d 项；未自动删除任何文件", t.Unknown, t.Retained)
	}
	if e = AtomicJSON(filepath.Join(dir, "summary.json"), t); e != nil {
		return nil, t, e
	}
	if e = ExportReports(dir, &p, t); e != nil {
		return nil, t, e
	}
	return &p, t, nil
}

func RecoveryPlan(ctx context.Context, dir string) (*Plan, error) {
	original, t, e := InspectRecovery(ctx, dir)
	if e != nil {
		return nil, e
	}
	p := *original
	p.Files = []File{}
	p.Bytes = 0
	p.Issues = []string{}
	p.Created = time.Now()
	p.RecoveryFrom = t.ID
	if t.Unknown > 0 || t.Retained > 0 {
		p.Issues = append(p.Issues, "中断任务存在待核对或双份文件，请先查看报告并人工处理")
	}
	for i, f := range original.Files {
		if t.States[i].State != "planned" {
			continue
		}
		info, e := os.Lstat(f.Source)
		if e != nil {
			return nil, e
		}
		f.info = info
		p.Files = append(p.Files, f)
		p.Bytes += f.Size
	}
	if len(p.Files) == 0 {
		p.Issues = append(p.Issues, "没有可继续移动的文件")
	}
	p.Existing, e = TargetEntries(ctx, p.Config.Target)
	if e != nil {
		return nil, e
	}
	p.ID = Fingerprint(&p)
	return &p, nil
}
func ValidateRecoveryPlan(ctx context.Context, p *Plan) error {
	for _, f := range p.Files {
		if e := CheckContext(ctx); e != nil {
			return e
		}
		if e := checkSource(f); e != nil {
			return e
		}
		if _, e := os.Lstat(f.Target); !os.IsNotExist(e) {
			return fmt.Errorf("恢复目标已存在或不可访问：%s", f.Target)
		}
	}
	entries, e := TargetEntries(ctx, p.Config.Target)
	if e != nil {
		return e
	}
	a, _ := json.Marshal(entries)
	b, _ := json.Marshal(p.Existing)
	if string(a) != string(b) {
		return errors.New("目标内容已变化，请重新生成恢复预览")
	}
	return nil
}

func RecordRecoveryResult(p *Plan, t Task) error {
	if p.RecoveryFrom == "" {
		return nil
	}
	if filepath.Base(p.RecoveryFrom) != p.RecoveryFrom {
		return errors.New("恢复来源标识无效")
	}
	dir := filepath.Join(filepath.Dir(t.Directory), p.RecoveryFrom)
	b, e := os.ReadFile(filepath.Join(dir, "plan.json"))
	if e != nil {
		return e
	}
	var original Plan
	if e = json.Unmarshal(b, &original); e != nil {
		return e
	}
	journal, e := os.OpenFile(filepath.Join(dir, "operations.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	indexes := map[string]int{}
	for i, f := range original.Files {
		indexes[f.Source] = i
	}
	for _, state := range t.States {
		index, ok := indexes[state.Source]
		if !ok || original.Files[index].Target != state.Target {
			journal.Close()
			return errors.New("恢复文件与原任务不一致")
		}
		event := struct {
			Time  time.Time `json:"time"`
			Index int       `json:"index"`
			Phase string    `json:"phase"`
			File  FileState `json:"file"`
		}{time.Now(), index, state.State, state}
		if e = json.NewEncoder(journal).Encode(event); e != nil {
			journal.Close()
			return e
		}
	}
	if e = journal.Sync(); e != nil {
		journal.Close()
		return e
	}
	if e = journal.Close(); e != nil {
		return e
	}
	_, _, e = InspectRecovery(context.Background(), dir)
	return e
}
