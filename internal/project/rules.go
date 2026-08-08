package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RulesTemplate 是项目规则模板在项目内的相对路径。
// 模板随 am init 下发，用户可以按项目改写；生成的指令文件永远由它派生。
const RulesTemplate = "templates/project-rules.md"

// WriteRules 把项目规则模板渲染成 filename —— 编排工具会自动读取的项目级指令文件。
//
// 各家 AI CLI 都从工作目录向上递归读取这类文件，但文件名互不相同，且写错时
// 不报错只静默失效，所以文件名由 tools.ProjectRulesFilename 解析后传进来。
//
// 模板缺失时返回空路径且不报错：老版本 am init 出来的项目没有这份模板，
// 缺规则不该阻断整条制作流程，由调用方降级告警。
//
// 目标已存在且内容不同时报错而不是覆盖：那多半是用户自己写的指令文件，
// 静默覆盖会连人带规则一起丢。这与 Initialize 拒绝覆盖异内容文件的行为一致。
func WriteRules(root, filename string) (string, error) {
	root, _ = filepath.Abs(root)
	source := filepath.Join(root, filepath.FromSlash(RulesTemplate))
	body, err := os.ReadFile(source)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 %s: %w", RulesTemplate, err)
	}

	destination := filepath.Join(root, filename)
	if err := rejectSymlinkPath(root, destination); err != nil {
		return "", err
	}
	existing, err := os.ReadFile(destination)
	if err == nil {
		if bytes.Equal(existing, body) {
			return destination, nil
		}
		return "", fmt.Errorf("%s 已存在且内容与 %s 不同：如果它是上一次生成的，删除后重跑；如果是你自己写的规则，把内容并进模板", destination, RulesTemplate)
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("检查 %s: %w", destination, err)
	}

	tmp, err := os.CreateTemp(root, ".am-rules-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(body); err == nil {
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
		return "", fmt.Errorf("写入 %s: %w", destination, err)
	}
	return destination, nil
}
