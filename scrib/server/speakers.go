package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"strings"
)

// matchSpeakers turns diarisation labels (SPEAKER_0, SPEAKER_1) into real
// speaker IDs by cosine-matching embeddings against the speakers table.
// Unknown speakers get persisted with an auto-generated name "Speaker N"
// (N = next unused integer for this meeting) so the UI always has something
// to show, and future meetings can match against these embeddings.
func (s *Server) matchSpeakers(ctx context.Context, meetingID int, segments []DiarizedSegment, embeddings []SpeakerEmbedding) []DiarizedSegment {
	if len(embeddings) == 0 {
		return segments
	}

	existing, err := s.loadSpeakerEmbeddings(ctx)
	if err != nil {
		log.Printf("matchSpeakers: load speakers: %v", err)
		return segments
	}

	// Label -> resolved (speakerID, displayName).
	type resolved struct {
		id   int
		name string
	}
	resolvedByLabel := map[string]resolved{}

	for _, emb := range embeddings {
		if len(emb.Embedding) == 0 {
			continue
		}
		bestID, bestName, bestScore := 0, "", -1.0
		for _, known := range existing {
			score := cosine(emb.Embedding, known.vec)
			if score > bestScore {
				bestScore = score
				bestID = known.id
				bestName = known.name
			}
		}
		const matchThreshold = 0.75
		if bestScore >= matchThreshold {
			resolvedByLabel[emb.Speaker] = resolved{id: bestID, name: bestName}
			// Refresh embedding with an exponential moving average so the
			// speaker's voiceprint tracks gradual changes (mic, room).
			updated := blend(existing[bestID].vec, emb.Embedding, 0.2)
			if err := s.updateSpeakerEmbedding(ctx, bestID, updated); err != nil {
				log.Printf("update speaker %d: %v", bestID, err)
			}
			continue
		}

		// No match — persist as a new speaker with a deterministic name.
		name := nextSpeakerName(existing)
		id, err := s.insertSpeaker(ctx, name, emb.Embedding)
		if err != nil {
			log.Printf("insert speaker %s: %v", name, err)
			continue
		}
		existing[id] = speakerRow{id: id, name: name, vec: emb.Embedding}
		resolvedByLabel[emb.Speaker] = resolved{id: id, name: name}
	}

	out := make([]DiarizedSegment, len(segments))
	for i, seg := range segments {
		out[i] = seg
		if r, ok := resolvedByLabel[seg.Speaker]; ok {
			id := r.id
			out[i].SpeakerID = &id
			out[i].Speaker = r.name
		}
	}
	return out
}

type speakerRow struct {
	id   int
	name string
	vec  []float32
}

func (s *Server) loadSpeakerEmbeddings(ctx context.Context) (map[int]speakerRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, embedding FROM speakers WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int]speakerRow{}
	for rows.Next() {
		var id int
		var name string
		var raw []byte
		if err := rows.Scan(&id, &name, &raw); err != nil {
			continue
		}
		vec, err := decodeEmbedding(raw)
		if err != nil || len(vec) == 0 {
			continue
		}
		out[id] = speakerRow{id: id, name: name, vec: vec}
	}
	return out, nil
}

func (s *Server) insertSpeaker(ctx context.Context, name string, vec []float32) (int, error) {
	raw, err := encodeEmbedding(vec)
	if err != nil {
		return 0, err
	}
	var id int
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO speakers (uuid, name, embedding, updated_at)
		 VALUES (gen_random_uuid()::text, $1, $2, NOW())
		 RETURNING id`, name, raw,
	).Scan(&id)
	return id, err
}

func (s *Server) updateSpeakerEmbedding(ctx context.Context, id int, vec []float32) error {
	raw, err := encodeEmbedding(vec)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE speakers SET embedding = $1, updated_at = NOW() WHERE id = $2`, raw, id)
	return err
}

// nextSpeakerName picks "Speaker N" where N is the next unused integer.
func nextSpeakerName(existing map[int]speakerRow) string {
	used := map[int]bool{}
	for _, r := range existing {
		var n int
		if _, err := scanSpeakerName(r.name, &n); err == nil {
			used[n] = true
		}
	}
	for i := 1; ; i++ {
		if !used[i] {
			return formatSpeakerName(i)
		}
	}
}

// scanSpeakerName parses "Speaker N" into N.
func scanSpeakerName(name string, n *int) (bool, error) {
	const prefix = "Speaker "
	if !strings.HasPrefix(name, prefix) {
		return false, nil
	}
	_, err := sscanOrErr(name[len(prefix):], n)
	return true, err
}

func sscanOrErr(s string, n *int) (int, error) {
	// Tiny int parser to avoid pulling fmt.Sscanf for one call.
	if len(s) == 0 {
		return 0, sql.ErrNoRows
	}
	v := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, sql.ErrNoRows
		}
		v = v*10 + int(c-'0')
	}
	*n = v
	return 1, nil
}

func formatSpeakerName(n int) string {
	// "Speaker 1", "Speaker 2", ...
	return "Speaker " + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// cosine returns cosine similarity in [-1, 1]. Zero for degenerate inputs.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// blend returns alpha*fresh + (1-alpha)*existing.
func blend(existing, fresh []float32, alpha float64) []float32 {
	if len(existing) != len(fresh) {
		return fresh
	}
	out := make([]float32, len(existing))
	for i := range existing {
		out[i] = float32(alpha*float64(fresh[i]) + (1-alpha)*float64(existing[i]))
	}
	return out
}

// encodeEmbedding serialises a []float32 as JSON bytes. Chose JSON over a
// packed binary format for debuggability; embeddings are small (256-512 floats).
func encodeEmbedding(vec []float32) ([]byte, error) { return json.Marshal(vec) }

func decodeEmbedding(raw []byte) ([]float32, error) {
	var vec []float32
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &vec); err != nil {
		return nil, err
	}
	return vec, nil
}
