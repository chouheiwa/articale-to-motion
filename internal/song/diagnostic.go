package song

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

// Keep provider output bounded and in memory only. Only the structured, redacted
// error is persisted; successful responses may contain signed URLs or audio hex.
type diagnosticBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *diagnosticBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := 65536 - len(b.data)
	if len(p) > remaining {
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}

type diagnostic struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var diagnosticRedactions = []*regexp.Regexp{
	regexp.MustCompile(`(?i)https?://[^\s<>"']+`),
	regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+[^\s,;"']+`),
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|access[_-]?token|refresh[_-]?token|token|secret|password)\s*[=:]\s*[^\s,;]+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`[A-Za-z0-9_./+=-]{32,}`),
}

func providerDiagnostic(raw []byte, env map[string]string) diagnostic {
	result := diagnostic{Message: "提供方未返回可解析的结构化错误（原始输出未保存）"}
	var envelope struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	raw = bytes.TrimSpace(raw)
	if json.Unmarshal(raw, &envelope) != nil || envelope.Error.Message == "" {
		return result
	}
	result.Code = envelope.Error.Code
	message := envelope.Error.Message
	for key, value := range env {
		upper := strings.ToUpper(key)
		if value != "" && (strings.Contains(upper, "KEY") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD")) {
			message = strings.ReplaceAll(message, value, "[REDACTED]")
		}
	}
	for _, pattern := range diagnosticRedactions {
		message = pattern.ReplaceAllString(message, "[REDACTED]")
	}
	// Remove terminal control characters as well as credentials.
	message = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, message)
	runes := []rune(message)
	if len(runes) > 2000 {
		message = string(runes[:2000]) + "…"
	}
	result.Message = message
	return result
}
