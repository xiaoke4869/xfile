package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"xfile/internal/core"
)

type FileService struct {
	app                  *application.App
	mu                   sync.Mutex
	plan                 *core.Plan
	scan                 *core.Snapshot
	scanKey              string
	targetKey            string
	targetEntries        []core.Entry
	previewCancel        context.CancelFunc
	latestRevision       int
	task                 core.Task
	taskCancel           context.CancelFunc
	taskRunning          bool
	fileOperationRunning bool
	fileOperationID      string
	fileTask             core.Task
	tasksPath            string
}

func newFileService() *FileService {
	base, e := os.UserConfigDir()
	if e != nil {
		base = os.TempDir()
	}
	return &FileService{tasksPath: filepath.Join(base, "xfile", "tasks")}
}

type Location struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
type HomeInfo struct {
	Home      string     `json:"home"`
	Locations []Location `json:"locations"`
	Separator string     `json:"separator"`
}

func (s *FileService) Home() (HomeInfo, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return HomeInfo{}, e
	}
	locations := []Location{{"个人目录", home}}
	for _, x := range []struct{ name, path string }{{"桌面", "Desktop"}, {"文稿", "Documents"}, {"下载", "Downloads"}} {
		p := filepath.Join(home, x.path)
		if i, e := os.Stat(p); e == nil && i.IsDir() {
			locations = append(locations, Location{x.name, p})
		}
	}
	return HomeInfo{home, locations, string(filepath.Separator)}, nil
}
func (s *FileService) List(path string) ([]core.Entry, error) {
	p, e := core.Canonical(path)
	if e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(p)
	if e != nil {
		return nil, e
	}
	out := []core.Entry{}
	for _, d := range entries {
		i, e := d.Info()
		if e != nil {
			return nil, e
		}
		out = append(out, core.Entry{Path: filepath.Join(p, d.Name()), Name: d.Name(), Dir: d.IsDir(), Size: i.Size(), Modified: i.ModTime().UnixMilli(), Link: d.Type()&os.ModeSymlink != 0})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
func (s *FileService) ChooseDirectory() (string, error) {
	return s.app.Dialog.OpenFile().SetTitle("选择目录").CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(false).PromptForSingleSelection()
}

type PreviewResult struct {
	*core.Plan
	FileCount     int `json:"fileCount"`
	ExistingCount int `json:"existingCount"`
	RenamedCount  int `json:"renamedCount"`
}

func result(p *core.Plan) PreviewResult {
	copy := *p
	renamed := 0
	for _, f := range p.Files {
		if f.Renamed {
			renamed++
		}
	}
	if len(copy.Files) > 500 {
		copy.Files = copy.Files[:500]
	}
	if len(copy.Existing) > 500 {
		copy.Existing = copy.Existing[:500]
	}
	return PreviewResult{&copy, len(p.Files), len(p.Existing), renamed}
}
func (s *FileService) validatePaths(c core.Config) (core.Config, error) {
	sources, e := core.ValidateSources(c.Sources)
	if e != nil {
		return c, e
	}
	target, e := core.ValidateTarget(sources, c.Target)
	if e != nil {
		return c, e
	}
	journal, e := core.Canonical(s.tasksPath)
	if e != nil {
		return c, e
	}
	for _, source := range sources {
		if core.PathsOverlap(source.Path, journal) {
			return c, errors.New("来源范围包含 XFile 任务日志，请选择更具体的业务目录")
		}
	}
	if core.PathsOverlap(target, journal) {
		return c, errors.New("目标不能包含或位于 XFile 任务日志目录")
	}
	c.Sources = sources
	c.Target = target
	return c, nil
}
func (s *FileService) Preview(config core.Config, revision int, refresh bool) (PreviewResult, error) {
	s.mu.Lock()
	if s.taskRunning || s.fileOperationRunning {
		s.mu.Unlock()
		return PreviewResult{}, errors.New("移动任务运行中，请等待任务结束后调整计划")
	}
	if revision < s.latestRevision {
		s.mu.Unlock()
		return PreviewResult{}, context.Canceled
	}
	s.latestRevision = revision
	if s.previewCancel != nil {
		s.previewCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.previewCancel = cancel
	b, _ := json.Marshal(struct {
		Sources []core.Source
		Target  string
	}{config.Sources, config.Target})
	key := string(b)
	snap := s.scan
	entries := s.targetEntries
	if refresh || key != s.scanKey {
		snap = nil
	}
	if refresh || config.Target != s.targetKey {
		entries = nil
	}
	s.plan = nil
	s.mu.Unlock()
	rawTarget := config.Target
	config, e := s.validatePaths(config)
	if e != nil {
		return PreviewResult{}, e
	}
	if snap == nil {
		snap, e = core.Scan(ctx, config.Sources, config.Target)
		if e != nil {
			return PreviewResult{}, e
		}
	}
	if entries == nil {
		target, err := core.Canonical(config.Target)
		if err != nil {
			return PreviewResult{}, err
		}
		entries, e = core.TargetEntries(ctx, target)
		if e != nil {
			return PreviewResult{}, e
		}
	}
	plan, e := core.MakePlan(ctx, snap, config, entries)
	if e != nil {
		return PreviewResult{}, e
	}
	plan.Revision = revision
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != s.latestRevision || s.taskRunning || s.fileOperationRunning || ctx.Err() != nil {
		return PreviewResult{}, context.Canceled
	}
	s.scan = snap
	s.scanKey = key
	s.targetKey = rawTarget
	s.targetEntries = entries
	s.plan = plan
	return result(plan), nil
}
func (s *FileService) PlanPage(id string, offset int, existing bool) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.plan == nil || s.plan.ID != id {
		return nil, errors.New("预览已失效")
	}
	if offset < 0 {
		return nil, errors.New("无效页码")
	}
	if existing {
		if offset > len(s.plan.Existing) {
			offset = len(s.plan.Existing)
		}
		end := min(offset+500, len(s.plan.Existing))
		return s.plan.Existing[offset:end], nil
	}
	if offset > len(s.plan.Files) {
		offset = len(s.plan.Files)
	}
	end := min(offset+500, len(s.plan.Files))
	return s.plan.Files[offset:end], nil
}
func (s *FileService) Start(id string, revision int) (string, error) {
	s.mu.Lock()
	if s.taskRunning || s.fileOperationRunning {
		s.mu.Unlock()
		return "", errors.New("已有任务运行中")
	}
	p := s.plan
	if p == nil || p.ID != id || p.Revision != revision || revision != s.latestRevision {
		s.mu.Unlock()
		return "", errors.New("预览已失效，请重新预览")
	}
	if len(p.Issues) > 0 {
		s.mu.Unlock()
		return "", errors.New("请先处理预览中的问题")
	}
	s.taskRunning = true
	ctx, cancel := context.WithCancel(context.Background())
	s.taskCancel = cancel
	taskID := time.Now().Format("20060102-150405.000000000")
	s.task = core.Task{ID: taskID, Status: "running", Phase: "执行前重新核对", Total: len(p.Files), Bytes: p.Bytes}
	s.mu.Unlock()
	go func() {
		var e error
		if p.RecoveryFrom != "" {
			e = core.ValidateRecoveryPlan(ctx, p)
		} else {
			var fresh *core.Plan
			fresh, e = core.BuildPlan(ctx, p.Config)
			if e == nil && fresh.ID != p.ID {
				e = errors.New("来源或目标已发生变化，请重新扫描并确认预览")
			}
		}
		if e != nil {
			s.mu.Lock()
			s.task.Status = "stale_plan"
			if errors.Is(e, context.Canceled) {
				s.task.Status = "cancelled"
			}
			s.task.Error = e.Error()
			s.task.Phase = "已停止"
			s.taskRunning = false
			s.scan = nil
			s.targetEntries = nil
			s.mu.Unlock()
			return
		}
		// Keep original file identities as an additional guard against replaced sources.
		t := core.Run(ctx, p, filepath.Join(s.tasksPath, taskID), func(t core.Task) {
			t.States = nil
			s.mu.Lock()
			s.task = t
			s.mu.Unlock()
		})
		if err := core.RecordRecoveryResult(p, t); err != nil {
			t.Status = "needs_review"
			t.Error += "；关联原任务日志失败：" + err.Error()
			_ = core.AtomicJSON(filepath.Join(t.Directory, "summary.json"), t)
		}
		s.mu.Lock()
		s.task = t
		s.taskRunning = false
		s.plan = nil
		s.scan = nil
		s.targetEntries = nil
		s.mu.Unlock()
	}()
	return taskID, nil
}
func (s *FileService) Task() core.Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.task
	if s.fileOperationRunning {
		t = s.fileTask
	}
	t.States = nil
	if s.taskRunning || s.fileOperationRunning {
		t.Status = "running"
	}
	return t
}
func (s *FileService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskCancel != nil {
		s.taskCancel()
	}
}
func (s *FileService) Tasks() ([]core.Task, error) {
	dirs, e := os.ReadDir(s.tasksPath)
	if os.IsNotExist(e) {
		return []core.Task{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []core.Task{}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		b, e := os.ReadFile(filepath.Join(s.tasksPath, d.Name(), "summary.json"))
		if e != nil {
			continue
		}
		var t core.Task
		if json.Unmarshal(b, &t) != nil {
			continue
		}
		s.normalizeTask(&t)
		t.States = nil
		t.Items = nil
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}
func (s *FileService) OpenReports(taskID string) error {
	if taskID == "" || filepath.Base(taskID) != taskID || taskID == "." || taskID == ".." {
		return errors.New("无效任务标识")
	}
	p := filepath.Join(s.tasksPath, taskID)
	if _, e := os.Stat(p); e != nil {
		return e
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", p).Run()
	case "windows":
		return exec.Command("explorer.exe", p).Start()
	}
	return fmt.Errorf("不支持的平台")
}
func (s *FileService) ExportReports(taskID string) (string, error) {
	if filepath.Base(taskID) != taskID || taskID == "" || taskID == "." || taskID == ".." {
		return "", errors.New("无效任务标识")
	}
	target, e := s.ChooseDirectory()
	if e != nil || target == "" {
		return "", e
	}
	dest := filepath.Join(target, "xfile-report-"+taskID)
	if e = os.Mkdir(dest, 0700); e != nil {
		return "", e
	}
	task, e := s.readTask(taskID)
	if e != nil {
		return dest, e
	}
	names := []string{"report.md", "summary.json"}
	if task.Operation == "" {
		names = append(names, "moves.csv", "renames.csv", "directory-names.csv", "operations.jsonl")
	}
	for _, name := range names {
		b, e := os.ReadFile(filepath.Join(s.tasksPath, taskID, name))
		if task.Operation != "" && name == "report.md" && os.IsNotExist(e) {
			continue // An interrupted operation may only have its saved intent.
		}
		if e != nil {
			return dest, e
		}
		if e = os.WriteFile(filepath.Join(dest, name), b, 0600); e != nil {
			return dest, e
		}
	}
	return dest, nil
}
func (s *FileService) InspectTask(id string) (core.Task, error) {
	if id == "" || filepath.Base(id) != id || id == "." || id == ".." {
		return core.Task{}, errors.New("无效任务标识")
	}
	s.mu.Lock()
	running := s.taskRunning || s.fileOperationRunning
	s.mu.Unlock()
	t, e := s.readTask(id)
	if e != nil || t.Operation != "" {
		return t, e
	}
	if running {
		return core.Task{}, errors.New("请等待当前任务结束")
	}
	_, t, e = core.InspectRecovery(context.Background(), filepath.Join(s.tasksPath, id))
	return t, e
}
func (s *FileService) PrepareRecovery(id string, revision int) (PreviewResult, error) {
	if id == "" || filepath.Base(id) != id || id == "." || id == ".." {
		return PreviewResult{}, errors.New("无效任务标识")
	}
	stored, err := s.readTask(id)
	if err != nil {
		return PreviewResult{}, err
	}
	if stored.Operation != "" {
		return PreviewResult{}, errors.New("此文件操作需手动核对，不支持归并恢复")
	}
	s.mu.Lock()
	if s.taskRunning || s.fileOperationRunning {
		s.mu.Unlock()
		return PreviewResult{}, errors.New("请等待当前任务结束")
	}
	if s.previewCancel != nil {
		s.previewCancel()
	}
	s.latestRevision = revision
	s.plan = nil
	s.mu.Unlock()
	p, e := core.RecoveryPlan(context.Background(), filepath.Join(s.tasksPath, id))
	if e != nil {
		return PreviewResult{}, e
	}
	p.Revision = revision
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != s.latestRevision || s.taskRunning || s.fileOperationRunning {
		return PreviewResult{}, errors.New("恢复预览已过期")
	}
	if _, err := os.Stat(filepath.Join(s.tasksPath, id, "summary.json")); err != nil {
		return PreviewResult{}, errors.New("原任务日志已删除，请重新选择任务")
	}
	s.plan = p
	return result(p), nil
}

func (s *FileService) DirectoryText(path string) (string, error) {
	return core.DirectoryText(context.Background(), path)
}

func (s *FileService) ExportDirectoryText(source string) (string, error) {
	root, err := core.Canonical(source)
	if err != nil {
		return "", err
	}
	destination, err := s.app.Dialog.SaveFile().SetFilename(filepath.Base(root)+"-目录结构.txt").AddFilter("文本文件", "*.txt").CanCreateDirectories(true).PromptForSingleSelection()
	if err != nil || destination == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(destination), ".txt") {
		destination += ".txt"
	}
	journal, err := core.Canonical(s.tasksPath)
	if err != nil {
		return "", err
	}
	resolved, err := core.Canonical(destination)
	if err != nil {
		return "", err
	}
	if core.PathsOverlap(resolved, journal) {
		return "", errors.New("不能导出到 XFile 任务日志范围内")
	}
	_, err = s.fileOperation("export_directory", []string{root}, destination, func() (core.OperationResult, error) {
		if err := core.WriteDirectoryText(context.Background(), root, destination); err != nil {
			return core.OperationResult{}, fmt.Errorf("保存失败，已有文件不会覆盖，请另选文件名：%w", err)
		}
		return core.OperationResult{Items: []core.OperationItem{{Source: root, Target: destination}}}, nil
	})
	return destination, err
}
