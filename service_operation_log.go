package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"xfile/internal/core"
)

// Save the intent before changing files, so an interrupted operation remains visible.
func (s *FileService) fileOperation(kind string, paths []string, target string, run func() (core.OperationResult, error)) (core.OperationResult, error) {
	if err := s.beginFileOperation(); err != nil {
		return core.OperationResult{}, err
	}
	defer s.finishFileOperation()
	now := time.Now()
	t := core.Task{ID: now.Format("20060102-150405.000000000"), Operation: kind, Status: "running", Phase: "正在处理", Started: now, Total: len(paths)}
	t.Directory = filepath.Join(s.tasksPath, t.ID)
	for _, path := range paths {
		t.Items = append(t.Items, core.OperationItem{Source: path, Target: target})
	}
	if err := os.MkdirAll(s.tasksPath, 0700); err != nil {
		return core.OperationResult{}, fmt.Errorf("无法创建操作日志，操作未执行：%w", err)
	}
	if err := os.Mkdir(t.Directory, 0700); err != nil {
		return core.OperationResult{}, err
	}
	if err := core.AtomicJSON(filepath.Join(t.Directory, "summary.json"), t); err != nil {
		return core.OperationResult{}, fmt.Errorf("无法保存操作日志，操作未执行：%w", err)
	}
	s.mu.Lock()
	s.fileOperationID = t.ID
	s.mu.Unlock()
	result, operationErr := run()
	if len(result.Items) > 0 {
		t.Items = result.Items
	}
	t.Total = len(t.Items)
	if operationErr != nil {
		t.Error = operationErr.Error()
	}
	for i := range t.Items {
		item := &t.Items[i]
		if operationErr != nil && item.Error == "" {
			item.Error = operationErr.Error()
		}
		if item.Error == "" {
			t.Moved++
		}
	}
	t.Status = "succeeded"
	if operationErr != nil || t.Moved < t.Total {
		t.Status = "failed"
		if t.Moved > 0 {
			t.Status = "partial_failed"
		}
	}
	t.Phase = "已结束"
	t.Finished = time.Now()
	if err := core.AtomicJSON(filepath.Join(t.Directory, "summary.json"), t); err != nil {
		return result, errors.Join(operationErr, fmt.Errorf("文件操作已结束，但保存结果日志失败，请核对文件：%w", err))
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# XFile 文件操作记录\n\n任务：%s\n\n操作：%s\n\n状态：%s\n\n时间：%s\n\n完成：%d / %d\n\n", t.ID, kind, t.Status, t.Started.Format(time.RFC3339), t.Moved, t.Total)
	for _, item := range t.Items {
		fmt.Fprintf(&report, "- 来源：%q\n  目标：%q\n  错误：%q\n\n", item.Source, item.Target, item.Error)
	}
	if t.Error != "" {
		fmt.Fprintf(&report, "错误：%s\n", t.Error)
	}
	if err := os.WriteFile(filepath.Join(t.Directory, "report.md"), []byte(report.String()), 0600); err != nil {
		return result, errors.Join(operationErr, fmt.Errorf("文件操作和结果已保存，但生成报告失败：%w", err))
	}
	return result, operationErr
}

func (s *FileService) normalizeTask(t *core.Task) {
	if t.Status != "running" {
		return
	}
	s.mu.Lock()
	active := s.taskRunning && s.task.ID == t.ID || s.fileOperationRunning && s.fileOperationID == t.ID
	s.mu.Unlock()
	if active {
		return
	}
	if t.Operation == "" {
		t.Status = "recovery_required"
	} else {
		t.Status = "needs_review"
		t.Unknown = t.Total
		t.Error = "操作中断，请根据来源和目标路径手动核对实际结果"
	}
}

func (s *FileService) readTask(id string) (core.Task, error) {
	var t core.Task
	if id == "" || filepath.Base(id) != id || id == "." || id == ".." {
		return t, errors.New("无效任务标识")
	}
	b, err := os.ReadFile(filepath.Join(s.tasksPath, id, "summary.json"))
	if err != nil {
		return t, err
	}
	if err = json.Unmarshal(b, &t); err != nil {
		return t, err
	}
	s.normalizeTask(&t)
	return t, nil
}

func (s *FileService) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskRunning || s.fileOperationRunning {
		return errors.New("请等待当前任务结束后删除日志")
	}
	return s.deleteTask(id)
}

func (s *FileService) ClearTasks() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskRunning || s.fileOperationRunning {
		return errors.New("请等待当前任务结束后清空日志")
	}
	entries, err := os.ReadDir(s.tasksPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var task core.Task
		b, err := os.ReadFile(filepath.Join(s.tasksPath, entry.Name(), "summary.json"))
		if err != nil || json.Unmarshal(b, &task) != nil || task.ID != entry.Name() {
			continue
		}
		if err := s.deleteTask(entry.Name()); err != nil {
			failures = append(failures, fmt.Errorf("%s：%w", entry.Name(), err))
		}
	}
	return errors.Join(failures...)
}

// Caller holds s.mu. Only remove the selected journal directory, never its recorded file paths.
func (s *FileService) deleteTask(id string) error {
	if id == "" || filepath.Base(id) != id || id == "." || id == ".." {
		return errors.New("无效任务标识")
	}
	dir := filepath.Join(s.tasksPath, id)
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("不是有效的日志目录")
	}
	var task core.Task
	b, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &task); err != nil {
		return err
	}
	if task.ID != id {
		return errors.New("日志标识不匹配")
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if s.task.ID == id {
		s.task = core.Task{}
	}
	if s.plan != nil && s.plan.RecoveryFrom == id {
		s.plan = nil
	}
	return nil
}
