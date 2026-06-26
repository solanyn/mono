package main

import (
	"context"
	"log"
	"sort"
)

// scrubText analyzes text via Presidio and replaces every detected PII span
// with its placeholder. Spans are replaced from end to start so the byte
// offsets of not-yet-replaced spans stay valid. It returns the (possibly)
// modified text and whether any replacement was made.
func scrubText(ctx context.Context, client *presidioClient, text string) (string, bool, error) {
	if text == "" {
		return text, false, nil
	}

	results, err := client.analyze(ctx, text)
	if err != nil {
		return text, false, err
	}
	if len(results) == 0 {
		return text, false, nil
	}

	// Replace from the highest start offset to the lowest so that each
	// replacement leaves the offsets of the remaining (lower) spans valid.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Start > results[j].Start
	})

	scrubbed := text
	changed := false
	// prevStart tracks the start offset of the last span we replaced. Any
	// span whose end extends into already-replaced territory overlaps it and
	// is skipped to avoid corrupting offsets.
	prevStart := len(text)
	for _, r := range results {
		if r.Start < 0 || r.End > len(text) || r.Start >= r.End {
			log.Printf("warning: skipping invalid span [%d:%d] for %s (text length %d)", r.Start, r.End, r.EntityType, len(text))
			continue
		}
		if r.End > prevStart {
			log.Printf("warning: skipping overlapping span [%d:%d] for %s", r.Start, r.End, r.EntityType)
			continue
		}
		scrubbed = scrubbed[:r.Start] + placeholderFor(r.EntityType) + scrubbed[r.End:]
		prevStart = r.Start
		changed = true
	}
	return scrubbed, changed, nil
}

// scrubContent scrubs a chat message "content" value, which may be either a
// plain string or a multimodal array of parts. For arrays, only parts with
// type "text" are scrubbed. It returns the modified content and whether any
// change was made.
func scrubContent(ctx context.Context, client *presidioClient, content interface{}) (interface{}, bool, error) {
	switch v := content.(type) {
	case string:
		return scrubText(ctx, client, v)
	case []interface{}:
		changed := false
		for _, part := range v {
			m, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t != "text" {
				continue
			}
			txt, ok := m["text"].(string)
			if !ok {
				continue
			}
			scrubbed, c, err := scrubText(ctx, client, txt)
			if err != nil {
				return content, false, err
			}
			if c {
				m["text"] = scrubbed
				changed = true
			}
		}
		return v, changed, nil
	default:
		return content, false, nil
	}
}
