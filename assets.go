package assets

import "embed"

// Files contains the reusable project skeleton shipped with the am binary.
//
//go:embed PROMPT.md PROMPT-PRODUCTION.md article-to-motion.conf .env.example frame.md templates/publish.md docs/清晰系统蓝图-视频风格说明书.md assets/style-guide/examples/*
var Files embed.FS
