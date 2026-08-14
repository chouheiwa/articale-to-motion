package assets

import (
	"embed"
	"fmt"
	"io/fs"
)

// Files 保存 am 二进制自带的项目骨架，分两棵源树：
//
//	assets/shared/    与画幅无关，所有预设共用
//	assets/presets/   含画幅数字，每套预设一份（由 go generate 产出）
//
// 每棵树的内部路径就是它在项目根下的目标路径，Initialize 直接叠加拷贝，
// 不做任何路径改写。
//
// assets/shared/assets/fonts 里的 CJK 字体必须随项目走：渲染机是干净的无头
// Chrome，字体栈里没有 @font-face 的字体族会静默回退，本地看着正常、成片排版
// 是错的。
//
// assets/shared/.agents/skills 是随项目下发的技能树。渲染工具靠 scene.ResolveSkill
// 从镜头目录逐级向上找 .agents/skills，落到项目根就能被发现，无需用户另行安装。
//
// .env.example 和 .agents 都要单列：//go:embed 对目录模式会跳过 . 开头的条目，
// 漏了不报错，只是整棵树静默消失——assets_test.go 的内容断言守着这一点。
//
//go:embed assets/shared assets/presets assets/shared/.env.example assets/shared/.agents
var Files embed.FS

// Shared 返回与画幅无关的那棵源树。
func Shared() (fs.FS, error) {
	return fs.Sub(Files, "assets/shared")
}

// Preset 返回指定画幅预设的素材树。
func Preset(id string) (fs.FS, error) {
	sub, err := fs.Sub(Files, "assets/presets/"+id)
	if err != nil {
		return nil, fmt.Errorf("找不到预设素材 %s: %w", id, err)
	}
	if _, err := fs.Stat(sub, "frame.md"); err != nil {
		return nil, fmt.Errorf("预设素材 %s 不完整，缺少 frame.md", id)
	}
	return sub, nil
}
