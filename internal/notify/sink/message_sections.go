package sink

import "strings"

func splitEvidenceMessage(msg string) (summary string, evidence string) {
	const marker = "\n[证据分析]\n"
	if strings.Contains(msg, marker) {
		parts := strings.SplitN(msg, marker, 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(msg), ""
}

func firstSuggestion(lines []string) string {
	if len(lines) == 0 {
		return "-"
	}
	return lines[0]
}
