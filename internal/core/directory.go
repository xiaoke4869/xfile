package core

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type DirectoryCount struct {
	Folders int   `json:"folders"`
	Files   int   `json:"files"`
	Bytes   int64 `json:"bytes"`
}

// CountDirectory includes hidden entries and sums regular file sizes without following links.
func CountDirectory(ctx context.Context, path string, progress func(DirectoryCount)) (DirectoryCount, error) {
	var count DirectoryCount
	root, err := Canonical(path)
	if err != nil {
		return count, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return count, err
	}
	if !info.IsDir() {
		return count, fmt.Errorf("来源不是目录：%s", root)
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if cancelled := CheckContext(ctx); cancelled != nil {
			return cancelled
		}
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			count.Folders++
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			count.Files++
			if info.Mode().IsRegular() {
				count.Bytes += info.Size()
			}
		}
		if progress != nil {
			progress(count)
		}
		return nil
	})
	return count, err
}

// DirectoryText lists names without following directory or file symlinks.
func DirectoryText(ctx context.Context, path string) (string, error) {
	root, err := Canonical(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("来源不是目录：%s", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	nested := false
	for _, entry := range entries {
		if entry.IsDir() {
			nested = true
			break
		}
	}
	var out strings.Builder
	if nested {
		out.WriteString(".\n")
	}
	escape := strings.NewReplacer("\\", "\\\\", "\r", "\\r", "\n", "\\n", "\t", "\\t")
	var write func(string, []fs.DirEntry, string, bool) error
	write = func(dir string, entries []fs.DirEntry, prefix string, tree bool) error {
		for index, entry := range entries {
			if err := CheckContext(ctx); err != nil {
				return err
			}
			last := index == len(entries)-1
			if tree {
				out.WriteString(prefix)
				if last {
					out.WriteString("└── ")
				} else {
					out.WriteString("├── ")
				}
			}
			out.WriteString(escape.Replace(entry.Name()))
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				out.WriteString(" [链接]")
			} else if !entry.IsDir() && !info.Mode().IsRegular() {
				out.WriteString(" [特殊文件]")
			}
			out.WriteByte('\n')
			if entry.IsDir() {
				path := filepath.Join(dir, entry.Name())
				if err := CheckParents(path); err != nil {
					return err
				}
				children, err := os.ReadDir(path)
				if err != nil {
					return err
				}
				indent := "│   "
				if last {
					indent = "    "
				}
				if err := write(path, children, prefix+indent, true); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := CheckContext(ctx); err != nil {
		return "", err
	}
	if err := write(root, entries, "", nested); err != nil {
		return "", err
	}
	return out.String(), nil
}

// WriteDirectoryText never replaces an existing file, including a symlink.
func WriteDirectoryText(ctx context.Context, source, destination string) error {
	destination, err := Canonical(destination)
	if err != nil {
		return err
	}
	text, err := DirectoryText(ctx, source)
	if err != nil {
		return err
	}
	if err = CheckParents(destination); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		file.Close()
		if !complete {
			os.Remove(destination)
		}
	}()
	if _, err = io.WriteString(file, text); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}
