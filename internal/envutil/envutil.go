// Package envutil 提供环境变量相关的共享工具函数，
// 消除 cli、config、scene 等包中的重复实现。
package envutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MissingToolPrefix 是「外部命令不存在」这类错误的固定前缀。
//
// cli.Execute 靠这个子串把错误映射成退出码 127（见根命令 --help 的退出码表），
// 所以它是跨包的契约而不是文案：任何解析外部命令失败的地方都必须用 LookPath，
// 或者至少复用这个前缀，否则调用方会把「没装 ffmpeg」当成一般失败处理。
const MissingToolPrefix = "缺少必需工具"

// LookPath 在给定的 PATH 值里查找可执行文件，不读取进程自身的 os.Getenv("PATH")。
//
// 不能用 exec.LookPath：安全模式下子进程的 PATH 是受控白名单，
// 与父进程的 PATH 可以完全不同，按父进程的 PATH 找到的二进制子进程未必能执行。
// 名字里已经带路径分隔符时原样返回，交由 exec 自己解析。
func LookPath(name, pathValue string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	for _, dir := range filepath.SplitList(pathValue) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s：%s", MissingToolPrefix, name)
}

// EnvMap 把 os.Environ() 转成 map。
func EnvMap() map[string]string {
	out := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

// EnvList 把 map 转成 "KEY=VALUE" 格式的 slice，供 exec.Cmd.Env 使用。
func EnvList(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

// ParsePassthrough 解析 AM_PASSTHROUGH_ENV 的逗号/空格分隔值。
func ParsePassthrough(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
}

// IsUnsafe 综合判断是否处于 unsafe 模式。
func IsUnsafe(flag bool) bool {
	return flag || os.Getenv("AM_UNSAFE") == "1"
}
