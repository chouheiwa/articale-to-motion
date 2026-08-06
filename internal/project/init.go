package project

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type InitResult struct {
	Created   int
	Unchanged int
}

type asset struct {
	name string
	body []byte
}

func Initialize(target string, source fs.FS) (InitResult, error) {
	target, _ = filepath.Abs(target)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("创建项目目录: %w", err)
	}
	var assets []asset
	err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || path == "assets.go" || path == "go.mod" {
			return nil
		}
		body, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		assets = append(assets, asset{name: path, body: body})
		return nil
	})
	if err != nil {
		return InitResult{}, fmt.Errorf("读取内置项目模板: %w", err)
	}

	result := InitResult{}
	for _, item := range assets {
		destination := filepath.Join(target, filepath.FromSlash(item.name))
		if err := rejectSymlinkPath(target, destination); err != nil {
			return InitResult{}, err
		}
		existing, err := os.ReadFile(destination)
		if err == nil {
			if !bytes.Equal(existing, item.body) {
				return InitResult{}, fmt.Errorf("目标文件已存在且内容不同：%s", destination)
			}
			result.Unchanged++
			continue
		}
		if !os.IsNotExist(err) {
			return InitResult{}, fmt.Errorf("检查目标文件 %s: %w", destination, err)
		}
	}

	var created []string
	rollback := func() {
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
	}
	for _, item := range assets {
		destination := filepath.Join(target, filepath.FromSlash(item.name))
		if _, err := os.Stat(destination); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			rollback()
			return InitResult{}, err
		}
		tmp, err := os.CreateTemp(filepath.Dir(destination), ".am-init-*")
		if err != nil {
			rollback()
			return InitResult{}, err
		}
		tmpName := tmp.Name()
		if _, err = tmp.Write(item.body); err == nil {
			err = tmp.Chmod(0o644)
		}
		if closeErr := tmp.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmpName, destination)
		}
		if err != nil {
			_ = os.Remove(tmpName)
			rollback()
			return InitResult{}, fmt.Errorf("写入 %s: %w", destination, err)
		}
		created = append(created, destination)
		result.Created++
	}
	return result, nil
}

func rejectSymlinkPath(root, destination string) error {
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == ".." || filepath.IsAbs(relative) {
		return fmt.Errorf("模板路径逃出项目目录：%s", destination)
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("检查模板路径 %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("模板路径不得包含符号链接：%s", current)
		}
		if current != destination && !info.IsDir() {
			return fmt.Errorf("模板父路径不是目录：%s", current)
		}
	}
	return nil
}
