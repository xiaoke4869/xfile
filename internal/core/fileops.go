package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type OperationItem struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Error  string `json:"error"`
}
type OperationResult struct {
	Items []OperationItem `json:"items"`
}

// SelectedPaths resolves aliases and collapses a selected parent and its children.
func SelectedPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, errors.New("请先选择文件或目录")
	}
	unique := []string{}
	for _, raw := range paths {
		if raw == "" {
			return nil, errors.New("无效来源路径")
		}
		info, err := os.Lstat(raw)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("请选择普通文件或目录：%s", raw)
		}
		p, err := Canonical(raw)
		if err != nil {
			return nil, err
		}
		if filepath.Dir(p) == p {
			return nil, errors.New("不能操作磁盘根目录")
		}
		skip := false
		for _, other := range unique {
			key := ProbeNameKey(other)
			if Within(key(other), key(p)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		kept := unique[:0]
		for _, other := range unique {
			key := ProbeNameKey(p)
			if !Within(key(p), key(other)) {
				kept = append(kept, other)
			}
		}
		unique = append(kept, p)
	}
	return unique, nil
}

// TransferEntries handles each top-level entry independently and never overwrites.
func TransferEntries(ctx context.Context, paths []string, target string, move bool) (OperationResult, error) {
	result := OperationResult{Items: []OperationItem{}}
	sources, err := SelectedPaths(paths)
	if err != nil {
		return result, err
	}
	target, err = Canonical(target)
	if err != nil {
		return result, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return result, err
	}
	if !info.IsDir() {
		return result, errors.New("目标不是目录")
	}
	for _, source := range sources {
		key := ProbeNameKey(source)
		if Within(key(source), key(target)) {
			return result, errors.New("目标不能是来源自身或来源的子目录")
		}
	}
	for _, source := range sources {
		item := OperationItem{Source: source}
		if err = CheckContext(ctx); err != nil {
			item.Error = err.Error()
		} else {
			item.Target, err = transferEntry(ctx, source, target, move)
			if err != nil {
				item.Error = err.Error()
			}
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func unusedEntryName(source, target string) (string, error) {
	entries, err := os.ReadDir(target)
	if err != nil {
		return "", err
	}
	key := ProbeNameKey(target)
	used := map[string]bool{}
	for _, e := range entries {
		used[key(e.Name())] = true
	}
	name := filepath.Base(source)
	ext := ""
	info, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		ext = filepath.Ext(name)
	}
	stem := name[:len(name)-len(ext)]
	candidate := name
	for n := 2; used[key(candidate)]; n++ {
		candidate = fmt.Sprintf("%s__%d%s", stem, n, ext)
	}
	return filepath.Join(target, candidate), nil
}

func transferEntry(ctx context.Context, source, target string, move bool) (string, error) {
	if err := CheckParents(source); err != nil {
		return "", err
	}
	if err := CheckParents(target); err != nil {
		return "", err
	}
	if move {
		key := ProbeNameKey(source)
		if key(filepath.Dir(source)) == key(target) {
			return "", errors.New("来源已经位于目标目录")
		}
		// Native exclusive rename also preserves empty directories and links inside them.
		for {
			destination, err := unusedEntryName(source, target)
			if err != nil {
				return "", err
			}
			if filepath.Clean(source) == destination {
				return "", errors.New("来源已经位于目标目录")
			}
			err = MoveExclusive(source, destination)
			if os.IsExist(err) {
				continue
			}
			if err == nil {
				return destination, nil
			}
			if !CrossDevice(err) {
				return "", err
			}
			break
		}
	}
	return transferByCopy(ctx, source, target, move)
}

func transferByCopy(ctx context.Context, source, target string, move bool) (string, error) {
	staging, err := os.MkdirTemp(target, ".xfile-transfer-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	staged := filepath.Join(staging, "item")
	nodes, err := copyEntry(ctx, source, staged)
	if err != nil {
		return "", err
	}
	if err = verifyEntry(ctx, source, nodes); err != nil {
		return "", err
	}
	var destination string
	for {
		if err = CheckContext(ctx); err != nil {
			return "", err
		}
		if err = CheckParents(target); err != nil {
			return "", err
		}
		destination, err = unusedEntryName(source, target)
		if err != nil {
			return "", err
		}
		err = MoveExclusive(staged, destination)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		break
	}
	if move {
		// Remove only verified entries; new or changed source content stays in place.
		if err = verifyEntry(ctx, source, nodes); err != nil {
			return destination, fmt.Errorf("目标已保存，来源已保留：%w", err)
		}
		for i := len(nodes) - 1; i >= 0; i-- {
			node := nodes[i]
			p := filepath.Join(source, node.relative)
			if err = checkEntryNode(ctx, p, node); err != nil {
				return destination, fmt.Errorf("目标已保存，部分来源已保留：%w", err)
			}
			if err = os.Remove(p); err != nil {
				return destination, fmt.Errorf("目标已保存，部分来源已保留：%w", err)
			}
		}
	}
	return destination, nil
}

type entryNode struct {
	relative string
	info     fs.FileInfo
	link     string
	hash     string
}

func entrySnapshot(ctx context.Context, source string) ([]entryNode, error) {
	nodes := []entryNode{}
	err := filepath.WalkDir(source, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := CheckContext(ctx); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("不支持特殊文件：%s", p)
		}
		relative, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		node := entryNode{relative: relative, info: info}
		if info.Mode()&os.ModeSymlink != 0 {
			node.link, err = os.Readlink(p)
			if err != nil {
				return err
			}
		}
		nodes = append(nodes, node)
		return nil
	})
	return nodes, err
}
func copyEntry(ctx context.Context, source, target string) ([]entryNode, error) {
	nodes, err := entrySnapshot(ctx, source)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		node := &nodes[i]
		from, to := filepath.Join(source, node.relative), filepath.Join(target, node.relative)
		if err = CheckContext(ctx); err != nil {
			return nil, err
		}
		if err = CheckParents(filepath.Dir(from)); err != nil {
			return nil, err
		}
		switch {
		case node.info.IsDir():
			err = os.Mkdir(to, 0755)
		case node.info.Mode()&os.ModeSymlink != 0:
			err = os.Symlink(node.link, to)
		default:
			node.hash, err = copyEntryFile(ctx, from, to, node.info)
		}
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}
func copyEntryFile(ctx context.Context, source, target string, info fs.FileInfo) (string, error) {
	src, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer src.Close()
	opened, err := src.Stat()
	if err != nil {
		return "", err
	}
	if !os.SameFile(opened, info) {
		return "", errors.New("来源在复制前被替换")
	}
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	hash := sha256.New()
	writer := io.MultiWriter(dst, hash)
	buffer := make([]byte, 256*1024)
	for {
		if err = CheckContext(ctx); err != nil {
			return "", err
		}
		n, readErr := src.Read(buffer)
		if n > 0 {
			if _, err = writer.Write(buffer[:n]); err != nil {
				return "", err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	if err = dst.Sync(); err != nil {
		return "", err
	}
	if err = dst.Close(); err != nil {
		return "", err
	}
	expected := hex.EncodeToString(hash.Sum(nil))
	actual, err := HashFile(ctx, target)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return "", errors.New("副本内容校验失败，来源已保留")
	}
	if err = os.Chmod(target, info.Mode().Perm()); err != nil {
		return "", err
	}
	if err = os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
		return "", err
	}
	return expected, nil
}
func checkEntryNode(ctx context.Context, p string, node entryNode) error {
	if err := CheckParents(filepath.Dir(p)); err != nil {
		return err
	}
	now, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !os.SameFile(now, node.info) || now.Mode() != node.info.Mode() {
		return fmt.Errorf("来源发生变化：%s", p)
	}
	if node.info.IsDir() {
		return nil
	}
	if now.Size() != node.info.Size() || !now.ModTime().Equal(node.info.ModTime()) {
		return fmt.Errorf("来源发生变化：%s", p)
	}
	if node.info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(p)
		if err != nil {
			return err
		}
		if link != node.link {
			return fmt.Errorf("链接发生变化：%s", p)
		}
	} else {
		sum, err := HashFile(ctx, p)
		if err != nil {
			return err
		}
		if sum != node.hash {
			return fmt.Errorf("来源内容发生变化：%s", p)
		}
	}
	return nil
}
func verifyEntry(ctx context.Context, source string, nodes []entryNode) error {
	current, err := entrySnapshot(ctx, source)
	if err != nil {
		return err
	}
	if len(current) != len(nodes) {
		return errors.New("来源目录内容发生变化")
	}
	for i, node := range nodes {
		if current[i].relative != node.relative {
			return errors.New("来源目录内容发生变化")
		}
		if err = checkEntryNode(ctx, filepath.Join(source, node.relative), node); err != nil {
			return err
		}
	}
	return nil
}
