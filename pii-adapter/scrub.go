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

	// Discard out-of-bounds / empty spans up front.
	valid := results[:0]
	for _, r := range results {
		if r.Start < 0 || r.End > len(text) || r.Start >= r.End {
			log.Printf("warning: skipping invalid span [%d:%d] for %s (text length %d)", r.Start, r.End, r.EntityType, len(text))
			continue
		}
		valid = append(valid, r)
	}
	if len(valid) == 0 {
		return text, false, nil
	}

	// Resolve overlaps in favour of the dominant span (mask more, not less):
	// sort by Start ascending, tie-breaking on End descending so the widest
	// span at a given start wins. Then greedily keep non-overlapping spans,
	// skipping any span that starts inside an already-selected span.
	sort.Slice(valid, func(i, j int) bool {
		if valid[i].Start != valid[j].Start {
			return valid[i].Start < valid[j].Start
		}
		return valid[i].End > valid[j].End
	})

	selected := valid[:0]
	lastEnd := -1
	for _, r := range valid {
		if r.Start < lastEnd {
			log.Printf("warning: skipping overlapping span [%d:%d] for %s (covered by span ending at %d)", r.Start, r.End, r.EntityType, lastEnd)
			continue
		}
		selected = append(selected, r)
		lastEnd = r.End
	}

	// Replace selected spans from end to start so that earlier (lower-offset)
	// replacements don't shift the offsets of spans not yet applied.
	scrubbed := text
	changed := false
	for i := len(selected) - 1; i >= 0; i-- {
		r := selected[i]
		scrubbed = scrubbed[:r.Start] + placeholderFor(r.EntityType) + scrubbed[r.End:]
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
