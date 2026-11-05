package main

import (
	"errors"
	"os"
	"path/filepath"
)

// OpenFile delegates a local file to the operating system's default application.
func (s *FileService) OpenFile(path string) error {
	return openLocalFile(path, func(target string) error {
		if s.app == nil {
			return errors.New("请在桌面客户端中打开文件")
		}
		return s.app.Browser.OpenFile(target)
	})
}

func openLocalFile(path string, launch func(string) error) error {
	if !filepath.IsAbs(path) {
		return errors.New("请选择有效的本地文件")
	}
	target := filepath.Clean(path)
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("只能打开普通文件，不支持目录、链接或特殊文件")
	}
	return launch(target)
}
