// Package fsutil 提供文件系统相关的共享工具函数。
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite 以原子方式写入文件：先写临时文件，再 rename 到目标路径。
// 写入失败时清理临时文件，不会留下半成品。
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".am-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(perm)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpName, path)
	}
	if err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("写入 %s: %w", path, err)
	}
	return nil
}
