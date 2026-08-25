package dialogue

import "path/filepath"

// Assemble 落地的三个产物路径，相对项目根。它们在写方（Assemble 本身）、
// 打印方（internal/cli 的 runDialogueAssemble）、读方（internal/validate/cast.go、
// internal/castbeats）之间只应有这一份真相源——原先三处各写一遍字面量，改
// 落地路径时读方极容易漏改，导致 am validate cast 报"找不到 production/dialogue.json"。
const (
	// SRTRelPath 是装配产出的字幕文件相对路径。
	SRTRelPath = "transcription-production.srt"
)

// VoiceRelPath 是装配产出的完整配音相对路径。
var VoiceRelPath = filepath.Join("production", "audio", "voice.wav")

// DialogueRelPath 是装配产出的结构化时间线相对路径。
// internal/castbeats.DialogueRelPath 是同一个值的别名，不要再造第三处。
var DialogueRelPath = filepath.Join("production", "dialogue.json")
