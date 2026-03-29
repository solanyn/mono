package scheduler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	shp "github.com/jonas-p/go-shp"
	"github.com/solanyn/mono/yield/api/internal/metrics"
)

const catchmentsURL = "https://data.nsw.gov.au/data/dataset/8b1e8161-7252-43d9-81ed-6311569cb1d7/resource/32d6f502-ddb1-45d9-b114-5e34ddfd33ac/download/catchments.zip"

func (s *Scheduler) syncCatchments() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Println("catchments: downloading catchment shapefiles")
	data, err := downloadURL(ctx, catchmentsURL)
	if err != nil {
		log.Printf("catchments: download: %v", err)
		return
	}
	log.Printf("catchments: downloaded %d KB", len(data)/1024)

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		log.Printf("catchments: zip open: %v", err)
		return
	}

	shpFiles := map[string]*zip.File{}
	dbfFiles := map[string]*zip.File{}
	for _, f := range r.File {
		lower := strings.ToLower(f.Name)
		base := lower
		if strings.HasSuffix(lower, ".shp") {
			base = strings.TrimSuffix(lower, ".shp")
			shpFiles[base] = f
		} else if strings.HasSuffix(lower, ".dbf") {
			base = strings.TrimSuffix(lower, ".dbf")
			dbfFiles[base] = f
		}
	}

	var total int64
	for base, shpFile := range shpFiles {
		dbfFile, ok := dbfFiles[base]
		if !ok {
			log.Printf("catchments: no .dbf for %s.shp, skipping", base)
			continue
		}

		count, err := s.processCatchmentShapefile(ctx, shpFile, dbfFile)
		if err != nil {
			log.Printf("catchments: process %s: %v", base, err)
			continue
		}
		total += count
		log.Printf("catchments: %s — %d catchments", base, count)
	}

	metrics.Global.SalesIngested.Add(total)
	log.Printf("catchments: sync complete — %d total catchments", total)
}

func (s *Scheduler) processCatchmentShapefile(ctx context.Context, shpZipFile, dbfZipFile *zip.File) (int64, error) {
	shpRC, err := shpZipFile.Open()
	if err != nil {
		return 0, fmt.Errorf("open shp: %w", err)
	}
	dbfRC, err := dbfZipFile.Open()
	if err != nil {
		shpRC.Close()
		return 0, fmt.Errorf("open dbf: %w", err)
	}

	sr := shp.SequentialReaderFromExt(io.NopCloser(shpRC), io.NopCloser(dbfRC))
	defer sr.Close()

	fields := sr.Fields()
	useIDIdx := fieldIndex(fields, "USE_ID")
	catchTypeIdx := fieldIndex(fields, "CATCH_TYPE")
	schoolIdx := fieldIndex(fields, "USE_DESC")
	priorityIdx := fieldIndex(fields, "PRIORITY")

	if schoolIdx < 0 {
		schoolIdx = fieldIndex(fields, "SCHOOL_NAM")
	}

	var count int64
	for sr.Next() {
		_, shape := sr.Shape()
		if shape == nil {
			continue
		}

		useID := ""
		if useIDIdx >= 0 {
			useID = sr.Attribute(useIDIdx)
		}
		catchType := ""
		if catchTypeIdx >= 0 {
			catchType = sr.Attribute(catchTypeIdx)
		} else {
			catchType = inferCatchType(shpZipFile.Name)
		}
		schoolName := ""
		if schoolIdx >= 0 {
			schoolName = sr.Attribute(schoolIdx)
		}
		priority := 0
		if priorityIdx >= 0 {
			fmt.Sscanf(sr.Attribute(priorityIdx), "%d", &priority)
		}

		wkb, err := shapeToWKB(shape)
		if err != nil {
			continue
		}

		if err := s.queries.UpsertCatchment(ctx, useID, catchType, schoolName, priority, wkb); err != nil {
			return count, fmt.Errorf("upsert catchment %s: %w", useID, err)
		}
		count++
	}

	if err := sr.Err(); err != nil {
		return count, fmt.Errorf("sequential read: %w", err)
	}

	return count, nil
}

func fieldIndex(fields []shp.Field, name string) int {
	for i, f := range fields {
		trimmed := strings.TrimRight(string(f.Name[:]), "\x00 ")
		if trimmed == name {
			return i
		}
	}
	return -1
}

func inferCatchType(filename string) string {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "primary") || strings.Contains(lower, "infants") {
		return "PRIMARY"
	}
	if strings.Contains(lower, "secondary") || strings.Contains(lower, "high") {
		return "SECONDARY"
	}
	if strings.Contains(lower, "future") {
		return "FUTURE"
	}
	return "UNKNOWN"
}

func shapeToWKB(shape shp.Shape) ([]byte, error) {
	switch s := shape.(type) {
	case *shp.Polygon:
		return polygonToMultiPolygonWKB(s.Points, s.Parts), nil
	case *shp.PolyLine:
		return polygonToMultiPolygonWKB(s.Points, s.Parts), nil
	default:
		return nil, fmt.Errorf("unsupported shape type: %T", shape)
	}
}

func polygonToMultiPolygonWKB(points []shp.Point, parts []int32) []byte {
	rings := splitRings(points, parts)

	var buf bytes.Buffer
	buf.WriteByte(1)
	binary.Write(&buf, binary.LittleEndian, uint32(6))
	binary.Write(&buf, binary.LittleEndian, uint32(1))

	buf.WriteByte(1)
	binary.Write(&buf, binary.LittleEndian, uint32(3))
	binary.Write(&buf, binary.LittleEndian, uint32(len(rings)))

	for _, ring := range rings {
		binary.Write(&buf, binary.LittleEndian, uint32(len(ring)))
		for _, pt := range ring {
			binary.Write(&buf, binary.LittleEndian, math.Float64bits(pt.X))
			binary.Write(&buf, binary.LittleEndian, math.Float64bits(pt.Y))
		}
	}

	return buf.Bytes()
}

func splitRings(points []shp.Point, parts []int32) [][]shp.Point {
	var rings [][]shp.Point
	for i, start := range parts {
		end := int32(len(points))
		if i+1 < len(parts) {
			end = parts[i+1]
		}
		rings = append(rings, points[start:end])
	}
	return rings
}
