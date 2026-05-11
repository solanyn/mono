package tracks

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
)

type TrackInfo struct {
	TrackID  string       `json:"track_id"`
	Name     string       `json:"name"`
	Country  string       `json:"country"`
	LengthM  float64      `json:"length_m"`
	Corners  []CornerInfo `json:"corners"`
	RefLineX []float64    `json:"reference_line_x,omitempty"`
	RefLineZ []float64    `json:"reference_line_z,omitempty"`
	Source   string       `json:"source"`
}

type CornerInfo struct {
	Number    int     `json:"number"`
	Name      string  `json:"name"`
	Direction string  `json:"direction"`
	ApexX     float64 `json:"reference_apex_x"`
	ApexZ     float64 `json:"reference_apex_z"`
	Notes     string  `json:"notes"`
}

func Fingerprint(x, z []float64, numPoints int) string {
	if numPoints <= 0 {
		numPoints = 64
	}
	n := len(x)
	if n == 0 {
		return ""
	}

	sampledX := make([]float64, numPoints)
	sampledZ := make([]float64, numPoints)
	for i := 0; i < numPoints; i++ {
		idx := int(float64(i) * float64(n-1) / float64(numPoints-1))
		if idx >= n {
			idx = n - 1
		}
		sampledX[i] = x[idx]
		sampledZ[i] = z[idx]
	}

	var cx, cz float64
	for i := 0; i < numPoints; i++ {
		cx += sampledX[i]
		cz += sampledZ[i]
	}
	cx /= float64(numPoints)
	cz /= float64(numPoints)

	for i := 0; i < numPoints; i++ {
		sampledX[i] -= cx
		sampledZ[i] -= cz
	}

	var maxDist float64
	for i := 0; i < numPoints; i++ {
		d := math.Sqrt(sampledX[i]*sampledX[i] + sampledZ[i]*sampledZ[i])
		if d > maxDist {
			maxDist = d
		}
	}
	if maxDist > 0 {
		for i := 0; i < numPoints; i++ {
			sampledX[i] /= maxDist
			sampledZ[i] /= maxDist
		}
	}

	quantized := make([]byte, 0, numPoints*16)
	for i := 0; i < numPoints; i++ {
		qx := math.Round(sampledX[i]*100) / 100
		qz := math.Round(sampledZ[i]*100) / 100
		quantized = append(quantized, []byte(fmt.Sprintf("%.2f,%.2f;", qx, qz))...)
	}

	h := sha256.Sum256(quantized)
	return fmt.Sprintf("%x", h[:8])
}

type Database struct {
	tracks map[string]*TrackInfo
}

func NewDatabase(tracksJSON []byte) *Database {
	db := &Database{tracks: make(map[string]*TrackInfo)}
	if len(tracksJSON) == 0 {
		return db
	}
	var data struct {
		Tracks []TrackInfo `json:"tracks"`
	}
	if err := json.Unmarshal(tracksJSON, &data); err != nil {
		return db
	}
	for i := range data.Tracks {
		db.tracks[data.Tracks[i].TrackID] = &data.Tracks[i]
	}
	return db
}

func (db *Database) Identify(x, z []float64) *TrackInfo {
	if len(x) < 100 {
		return nil
	}

	fp := Fingerprint(x, z, 64)

	for _, t := range db.tracks {
		if len(t.RefLineX) == 0 {
			continue
		}
		refFP := Fingerprint(t.RefLineX, t.RefLineZ, 64)
		if refFP == fp {
			return t
		}
	}

	queryNorm := normalize(x, z, 128)
	var bestMatch *TrackInfo
	bestScore := math.MaxFloat64

	for _, t := range db.tracks {
		if len(t.RefLineX) == 0 {
			continue
		}
		refNorm := normalize(t.RefLineX, t.RefLineZ, 128)
		score := hausdorffApprox(queryNorm, refNorm)
		if score < bestScore {
			bestScore = score
			bestMatch = t
		}
	}

	if bestScore < 0.15 {
		return bestMatch
	}
	return nil
}

func (db *Database) Get(trackID string) *TrackInfo {
	return db.tracks[trackID]
}

func (db *Database) LearnTrack(name string, x, z []float64) *TrackInfo {
	fp := Fingerprint(x, z, 64)
	var length float64
	for i := 1; i < len(x); i++ {
		dx := x[i] - x[i-1]
		dz := z[i] - z[i-1]
		length += math.Sqrt(dx*dx + dz*dz)
	}
	info := &TrackInfo{
		TrackID:  fp,
		Name:     name,
		LengthM:  length,
		RefLineX: x,
		RefLineZ: z,
		Source:   "learned",
	}
	db.tracks[fp] = info
	return info
}

func normalize(x, z []float64, numPoints int) [][2]float64 {
	n := len(x)
	pts := make([][2]float64, numPoints)
	for i := 0; i < numPoints; i++ {
		idx := int(float64(i) * float64(n-1) / float64(numPoints-1))
		if idx >= n {
			idx = n - 1
		}
		pts[i] = [2]float64{x[idx], z[idx]}
	}

	var cx, cz float64
	for _, p := range pts {
		cx += p[0]
		cz += p[1]
	}
	cx /= float64(numPoints)
	cz /= float64(numPoints)

	var maxDist float64
	for i := range pts {
		pts[i][0] -= cx
		pts[i][1] -= cz
		d := math.Sqrt(pts[i][0]*pts[i][0] + pts[i][1]*pts[i][1])
		if d > maxDist {
			maxDist = d
		}
	}
	if maxDist > 0 {
		for i := range pts {
			pts[i][0] /= maxDist
			pts[i][1] /= maxDist
		}
	}
	return pts
}

func hausdorffApprox(a, b [][2]float64) float64 {
	forward := meanMinDist(a, b)
	backward := meanMinDist(b, a)
	if forward > backward {
		return forward
	}
	return backward
}

func meanMinDist(a, b [][2]float64) float64 {
	var total float64
	for _, pa := range a {
		minD := math.MaxFloat64
		for _, pb := range b {
			dx := pa[0] - pb[0]
			dz := pa[1] - pb[1]
			d := dx*dx + dz*dz
			if d < minD {
				minD = d
			}
		}
		total += math.Sqrt(minD)
	}
	return total / float64(len(a))
}
