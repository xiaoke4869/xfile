package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"xfile/internal/core"
)

// CountDirectory runs only on request; intermediate counts are limited to ten updates per second.
func (s *FileService) CountDirectory(ctx context.Context, path, eventName string) (core.DirectoryCount, error) {
	if !strings.HasPrefix(eventName, "directory-count:") || len(eventName) > 100 {
		return core.DirectoryCount{}, errors.New("无效的统计请求")
	}
	last := time.Now()
	return core.CountDirectory(ctx, path, func(count core.DirectoryCount) {
		if s.app != nil && time.Since(last) >= 100*time.Millisecond {
			s.app.Event.Emit(eventName, count)
			last = time.Now()
		}
	})
}

func (s *FileService) beginFileOperation() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskRunning || s.fileOperationRunning {
		return errors.New("请等待当前文件操作结束")
	}
	s.fileOperationRunning = true
	if s.previewCancel != nil {
		s.previewCancel()
	}
	s.plan = nil
	s.scan = nil
	s.targetEntries = nil
	s.scanKey = ""
	s.targetKey = ""
	return nil
}
func (s *FileService) finishFileOperation() {
	s.mu.Lock()
	s.fileOperationRunning = false
	s.fileOperationID = ""
	s.mu.Unlock()
}
func (s *FileService) operationPaths(paths []string) ([]string, error) {
	sources, err := core.SelectedPaths(paths)
	if err != nil {
		return nil, err
	}
	journal, err := core.Canonical(s.tasksPath)
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		if core.PathsOverlap(source, journal) {
			return nil, errors.New("不能操作 XFile 任务日志或包含日志的目录")
		}
	}
	return sources, nil
}
func (s *FileService) TransferEntries(paths []string, target string, move bool) (core.OperationResult, error) {
	kind := "copy"
	if move {
		kind = "move"
	}
	return s.fileOperation(kind, paths, target, func() (core.OperationResult, error) {
		sources, err := s.operationPaths(paths)
		if err != nil {
			return core.OperationResult{}, err
		}
		destination, err := core.Canonical(target)
		if err != nil {
			return core.OperationResult{}, err
		}
		journal, err := core.Canonical(s.tasksPath)
		if err != nil {
			return core.OperationResult{}, err
		}
		if key := core.ProbeNameKey(journal); core.Within(key(journal), key(destination)) {
			return core.OperationResult{}, errors.New("目标不能包含或位于 XFile 任务日志目录")
		}
		return core.TransferEntries(context.Background(), sources, destination, move)
	})
}
func (s *FileService) DeleteEntries(paths []string) (core.OperationResult, error) {
	return s.fileOperation("delete", paths, "", func() (core.OperationResult, error) {
		sources, err := s.operationPaths(paths)
		if err != nil {
			return core.OperationResult{}, err
		}
		return trashEntries(sources, core.TrashEntry), nil
	})
}

func (s *FileService) renamePaths(paths []string) error {
	for _, path := range paths {
		if _, err := s.operationPaths([]string{path}); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileService) PreviewRename(paths []string, config core.RenameConfig) (core.RenamePreview, error) {
	if err := s.renamePaths(paths); err != nil {
		return core.RenamePreview{}, err
	}
	p, err := core.PlanRenames(paths, config)
	if len(p.Items) > 10 {
		p.Items = p.Items[:10]
	}
	if len(p.Issues) > 10 {
		p.Issues = append(p.Issues[:10], "其余问题请调整规则后重新预览")
	}
	return p, err
}

func (s *FileService) RenameEntries(paths []string, config core.RenameConfig) (core.OperationResult, error) {
	return s.fileOperation("rename", paths, "", func() (core.OperationResult, error) {
		if err := s.renamePaths(paths); err != nil {
			return core.OperationResult{}, err
		}
		return core.RenameEntries(paths, config)
	})
}

func trashEntries(paths []string, trash func(string) error) core.OperationResult {
	result := core.OperationResult{Items: []core.OperationItem{}}
	for _, source := range paths {
		item := core.OperationItem{Source: source}
		if err := core.CheckParents(source); err != nil {
			item.Error = err.Error()
		} else if err = trash(source); err != nil {
			item.Error = err.Error()
		} else if _, err = os.Lstat(source); !os.IsNotExist(err) {
			item.Error = fmt.Sprintf("无法确认项目已移入回收站：%s", source)
		}
		result.Items = append(result.Items, item)
	}
	return result
}

// CreateFolder creates one folder in the current directory without replacing an existing entry.
func (s *FileService) CreateFolder(directory, name string) (string, error) {
	result, err := s.fileOperation("create_folder", []string{directory}, filepath.Join(directory, name), func() (core.OperationResult, error) {
		if strings.TrimSpace(name) == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\`+"\x00") {
			return core.OperationResult{}, errors.New("请输入有效的文件夹名称，不能包含路径分隔符")
		}
		destination, err := core.Canonical(directory)
		if err != nil {
			return core.OperationResult{}, err
		}
		target := filepath.Join(destination, name)
		if err = core.CheckParents(target); err != nil {
			return core.OperationResult{}, err
		}
		journal, err := core.Canonical(s.tasksPath)
		if err != nil {
			return core.OperationResult{}, err
		}
		key := core.ProbeNameKey(journal)
		if core.Within(key(journal), key(target)) {
			return core.OperationResult{}, errors.New("不能在 XFile 任务日志目录内新建文件夹")
		}
		if err = os.Mkdir(target, 0755); err != nil {
			return core.OperationResult{}, err
		}
		return core.OperationResult{Items: []core.OperationItem{{Source: destination, Target: target}}}, nil
	})
	if err != nil {
		return "", err
	}
	return result.Items[0].Target, nil
}

// EntryInfo reads metadata for the current directory's information dialog.
func (s *FileService) EntryInfo(path string) (core.Entry, error) {
	target, err := core.Canonical(path)
	if err != nil {
		return core.Entry{}, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return core.Entry{}, err
	}
	return core.Entry{
		Path: target, Name: filepath.Base(target), Dir: info.IsDir(), Size: info.Size(),
		Modified: info.ModTime().UnixMilli(), Link: info.Mode()&os.ModeSymlink != 0,
	}, nil
}
