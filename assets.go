package assets

import "embed"

// Files contains the reusable project skeleton shipped with the am binary.
//
// assets/fonts holds the CJK faces the compositions must ship themselves: the
// render machine is a clean headless Chrome, so a system font named in the
// stack would silently fall back and break typography in the MP4.
//
//go:embed PROMPT.md PROMPT-PRODUCTION.md article-to-motion.conf .env.example frame.md templates/publish.md docs/清晰系统蓝图-视频风格说明书.md assets/style-guide/examples/* assets/fonts/*
var Files embed.FS
