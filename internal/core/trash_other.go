//go:build !darwin && !windows

package core

import "errors"

func TrashEntry(path string) error { return errors.New("当前平台暂不支持移入回收站") }
