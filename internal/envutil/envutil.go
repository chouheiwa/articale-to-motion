// Package envutil 提供环境变量相关的共享工具函数，
// 消除 cli、config、scene 等包中的重复实现。
package envutil

import (
	"os"
	"strings"
)

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
