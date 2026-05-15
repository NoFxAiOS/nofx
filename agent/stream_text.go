package agent

import "strings"

func emitStreamText(onEvent func(event, data string), text string) {
	if onEvent == nil {
		return
	}
	for _, chunk := range splitStreamText(text) {
		onEvent(StreamEventDelta, chunk)
	}
}

func splitStreamText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	chunks := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		start := 0
		for i, r := range line {
			switch r {
			case '。', '！', '？', '.', '!', '?', ';', '；', '：', ':', '，', ',':
				part := strings.TrimSpace(line[start : i+len(string(r))])
				if part != "" {
					chunks = append(chunks, part)
				}
				start = i + len(string(r))
			}
		}
		if start < len(line) {
			part := strings.TrimSpace(line[start:])
			if part != "" {
				chunks = append(chunks, part)
			}
		}
	}
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}
