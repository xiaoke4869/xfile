package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type RenameConfig struct {
	Mode     string `json:"mode"`
	Name     string `json:"name"`
	Find     string `json:"find"`
	Replace  string `json:"replace"`
	Text     string `json:"text"`
	Format   string `json:"format"`
	Position string `json:"position"`
	Start    int64  `json:"start"`
}

type RenamePreview struct {
	Items   []OperationItem `json:"items"`
	Total   int             `json:"total"`
	Changed int             `json:"changed"`
	Issues  []string        `json:"issues"`
}

func renamedName(name string, dir bool, index int, c RenameConfig) string {
	if c.Mode == "single" {
		return c.Name
	}
	ext := ""
	if !dir {
		ext = filepath.Ext(name)
		if ext == name {
			ext = ""
		}
	}
	stem := strings.TrimSuffix(name, ext)
	switch c.Mode {
	case "replace":
		stem = strings.ReplaceAll(stem, c.Find, c.Replace)
	case "add":
		if c.Position == "before" {
			stem = c.Text + stem
		} else {
			stem += c.Text
		}
	case "format":
		stem = strings.ReplaceAll(c.Format, "<原文件名>", stem)
		number := strconv.FormatInt(c.Start+int64(index), 10)
		if c.Position == "before" {
			stem = number + stem
		} else {
			stem += number
		}
	}
	return stem + ext
}

// PlanRenames is read-only and retains selection order, including selected children.
func PlanRenames(paths []string, c RenameConfig) (RenamePreview, error) {
	p := RenamePreview{Items: []OperationItem{}, Issues: []string{}, Total: len(paths)}
	if len(paths) == 0 {
		return p, errors.New("请先选择文件或目录")
	}
	switch c.Mode {
	case "single":
		if len(paths) != 1 {
			return p, errors.New("单项重命名只能选择一个项目")
		}
	case "replace":
		if c.Find == "" {
			return p, errors.New("请输入要查找的文本")
		}
	case "add":
		if c.Text == "" {
			return p, errors.New("请输入要添加的文本")
		}
	case "format":
		if c.Format == "" {
			return p, errors.New("请输入自定义格式")
		}
		if c.Start < 0 || c.Start > 9007199254740991-int64(len(paths)-1) {
			return p, errors.New("开始数字须为非负整数，且编号不能超过 9007199254740991")
		}
	default:
		return p, errors.New("无效的重命名模式")
	}
	if (c.Mode == "add" || c.Mode == "format") && c.Position != "before" && c.Position != "after" {
		return p, errors.New("请选择名称之前或之后")
	}
	seen, targets := map[string]bool{}, map[string]bool{}
	directoryKeys := map[string]func(string) string{}
	directoryNames := map[string]map[string]int{}
	for i, raw := range paths {
		sources, err := SelectedPaths([]string{raw})
		if err != nil {
			return p, err
		}
		source := sources[0]
		if err = CheckParents(source); err != nil {
			return p, err
		}
		info, err := os.Lstat(source)
		if err != nil {
			return p, err
		}
		name := renamedName(filepath.Base(source), info.IsDir(), i, c)
		item := OperationItem{Source: source, Target: filepath.Join(filepath.Dir(source), name)}
		parent := filepath.Dir(source)
		key := directoryKeys[parent]
		if key == nil {
			key = ProbeNameKey(parent)
			directoryKeys[parent] = key
			entries, err := os.ReadDir(parent)
			if err != nil {
				return p, err
			}
			directoryNames[parent] = map[string]int{}
			for _, entry := range entries {
				directoryNames[parent][key(entry.Name())]++
			}
		}
		if seen[key(source)] {
			return p, errors.New("重复选择了同一个项目")
		}
		seen[key(source)] = true
		if err = ValidName(name); err != nil {
			item.Error = err.Error()
		} else if targets[key(item.Target)] {
			item.Error = "多个项目将使用同一名称"
		} else {
			count := directoryNames[parent][key(name)]
			if count > 1 || count == 1 && key(name) != key(filepath.Base(source)) {
				item.Error = "目标名称已存在"
			}
		}
		targets[key(item.Target)] = true
		if item.Error != "" {
			p.Issues = append(p.Issues, filepath.Base(source)+"："+item.Error)
		}
		if source != item.Target {
			p.Changed++
		}
		p.Items = append(p.Items, item)
	}
	return p, nil
}

func RenameEntries(paths []string, c RenameConfig) (OperationResult, error) {
	p, err := PlanRenames(paths, c)
	result := OperationResult{Items: p.Items}
	if err != nil {
		return result, err
	}
	if len(p.Issues) != 0 {
		return result, errors.New(strings.Join(p.Issues, "\n"))
	}
	order := make([]int, len(p.Items))
	for i := range order {
		order[i] = i
	}
	// Rename children before their parents, without changing numbering order.
	sort.SliceStable(order, func(i, j int) bool { return len(p.Items[order[i]].Source) > len(p.Items[order[j]].Source) })
	for _, i := range order {
		item := &result.Items[i]
		if item.Source == item.Target {
			continue
		}
		if err = CheckParents(item.Source); err == nil {
			key := ProbeNameKey(filepath.Dir(item.Source))
			if key(item.Source) == key(item.Target) {
				err = renameCase(item.Source, item.Target)
			} else {
				err = MoveExclusive(item.Source, item.Target)
			}
		}
		if err != nil {
			item.Error = err.Error()
			continue
		}
		for j := range result.Items {
			child := &result.Items[j]
			if j != i && Within(item.Source, child.Target) {
				rel, _ := filepath.Rel(item.Source, child.Target)
				child.Target = filepath.Join(item.Target, rel)
			}
		}
	}
	return result, nil
}

func renameCase(source, target string) error {
	temp, err := os.MkdirTemp(filepath.Dir(source), ".xfile-rename-")
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	staged := filepath.Join(temp, "item")
	if err = MoveExclusive(source, staged); err != nil {
		return err
	}
	if err = MoveExclusive(staged, target); err != nil {
		if rollback := MoveExclusive(staged, source); rollback != nil {
			return fmt.Errorf("重命名失败：%v；原项目保留在 %s（恢复失败：%v）", err, staged, rollback)
		}
	}
	return err
}
