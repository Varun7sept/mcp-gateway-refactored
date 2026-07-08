package ai

import "strings"

func (m *Manager) stripThink(s string) string {
	return strings.TrimSpace(m.thinkRegex.ReplaceAllString(s, ""))
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
