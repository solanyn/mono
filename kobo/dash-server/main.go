package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

var (
	addr   string
	width  int
	height int
	mdFile string
	binDir string
)

func init() {
	flag.StringVar(&addr, "addr", envOr("DASH_ADDR", ":8080"), "listen address")
	flag.IntVar(&width, "width", envOrInt("DASH_WIDTH", 1072), "display width")
	flag.IntVar(&height, "height", envOrInt("DASH_HEIGHT", 1448), "display height")
	flag.StringVar(&mdFile, "file", envOr("DASH_FILE", "/data/dashboard.md"), "markdown file path")
	flag.StringVar(&binDir, "bindir", envOr("DASH_BINDIR", "/data/bin/"), "directory containing kobo-dash binary")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

var (
	headerFace  font.Face
	sectionFace font.Face
	bodyFace    font.Face
	boldFace    font.Face
	quoteFace   font.Face
)

var (
	boldRe  = regexp.MustCompile(`\*\*(.+?)\*\*`)
	emojiRe = regexp.MustCompile(`[\x{1F000}-\x{1FFFF}]|[\x{2600}-\x{27BF}]|[\x{FE00}-\x{FEFF}]|[\x{1F900}-\x{1F9FF}]|[\x{200D}\x{20E3}\x{FE0F}]`)
	warnRe  = regexp.MustCompile(`\x{26A0}\x{FE0F}?`)
)

func initFonts() {
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		log.Fatalf("parse bold font: %v", err)
	}
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		log.Fatalf("parse regular font: %v", err)
	}

	headerFace, err = opentype.NewFace(bold, &opentype.FaceOptions{Size: 36, DPI: 300, Hinting: font.HintingFull})
	if err != nil {
		log.Fatalf("header face: %v", err)
	}
	sectionFace, err = opentype.NewFace(bold, &opentype.FaceOptions{Size: 24, DPI: 300, Hinting: font.HintingFull})
	if err != nil {
		log.Fatalf("section face: %v", err)
	}
	bodyFace, err = opentype.NewFace(regular, &opentype.FaceOptions{Size: 18, DPI: 300, Hinting: font.HintingFull})
	if err != nil {
		log.Fatalf("body face: %v", err)
	}
	boldFace, err = opentype.NewFace(bold, &opentype.FaceOptions{Size: 18, DPI: 300, Hinting: font.HintingFull})
	if err != nil {
		log.Fatalf("bold face: %v", err)
	}
	quoteFace, err = opentype.NewFace(regular, &opentype.FaceOptions{Size: 16, DPI: 300, Hinting: font.HintingFull})
	if err != nil {
		log.Fatalf("quote face: %v", err)
	}
}

func main() {
	flag.Parse()
	initFonts()
	http.HandleFunc("/dashboard.png", handleDashboard)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/kobo-dash.bin", handleBinary)
	http.HandleFunc("/kobo-dash.sha256", handleBinarySHA256)
	log.Printf("listening on %s (%dx%d) file=%s bindir=%s", addr, width, height, mdFile, binDir)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

var (
	binHashCache   string
	binHashModTime time.Time
	binHashMu      sync.Mutex
)

func binPath() string {
	return filepath.Join(binDir, "kobo-dash")
}

func computeBinHash() (string, error) {
	binHashMu.Lock()
	defer binHashMu.Unlock()

	info, err := os.Stat(binPath())
	if err != nil {
		return "", err
	}

	if binHashCache != "" && info.ModTime().Equal(binHashModTime) {
		return binHashCache, nil
	}

	data, err := os.ReadFile(binPath())
	if err != nil {
		return "", err
	}

	h := sha256.Sum256(data)
	binHashCache = hex.EncodeToString(h[:])
	binHashModTime = info.ModTime()
	return binHashCache, nil
}

func handleBinary(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, binPath())
}

func handleBinarySHA256(w http.ResponseWriter, r *http.Request) {
	hash, err := computeBinHash()
	if err != nil {
		http.Error(w, "binary not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, hash)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile(mdFile)
	if err != nil {
		log.Printf("read %s: %v", mdFile, err)
		content = nil
	}

	img := renderMarkdown(width, height, content)
	gray := toGrayscale(img)
	dithered := floydSteinberg(gray)

	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, dithered); err != nil {
		log.Printf("png encode: %v", err)
	}
}

func cleanEmoji(s string) string {
	s = warnRe.ReplaceAllString(s, "[!]")
	s = emojiRe.ReplaceAllString(s, "")
	return s
}

type span struct {
	text string
	bold bool
}

func parseInline(s string) []span {
	s = cleanEmoji(s)
	var spans []span
	for {
		loc := boldRe.FindStringIndex(s)
		if loc == nil {
			if s != "" {
				spans = append(spans, span{text: s})
			}
			break
		}
		if loc[0] > 0 {
			spans = append(spans, span{text: s[:loc[0]]})
		}
		match := boldRe.FindStringSubmatch(s[loc[0]:])
		spans = append(spans, span{text: match[1], bold: true})
		s = s[loc[1]:]
	}
	return spans
}

func drawSpans(dc *gg.Context, spans []span, x, y float64) {
	for _, sp := range spans {
		if sp.bold {
			dc.SetFontFace(boldFace)
		} else {
			dc.SetFontFace(bodyFace)
		}
		dc.DrawString(sp.text, x, y)
		w, _ := dc.MeasureString(sp.text)
		x += w
	}
}

func renderMarkdown(w, h int, content []byte) image.Image {
	dc := gg.NewContext(w, h)
	dc.SetColor(color.White)
	dc.Clear()
	dc.SetColor(color.Black)

	if content == nil {
		dc.SetFontFace(sectionFace)
		dc.DrawStringAnchored("No dashboard data", float64(w)/2, float64(h)/2, 0.5, 0.5)
		return dc.Image()
	}

	margin := 60.0
	y := 80.0
	contentW := float64(w) - 2*margin
	lines := strings.Split(string(content), "\n")
	firstSection := true

	for _, line := range lines {
		if y > float64(h)-40 {
			break
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			y += 15
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## "):
			text := cleanEmoji(strings.TrimPrefix(trimmed, "# "))
			dc.SetFontFace(headerFace)
			dc.SetColor(color.Black)
			dc.DrawStringAnchored(text, float64(w)/2, y, 0.5, 0.5)
			y += 70

		case strings.HasPrefix(trimmed, "## "):
			text := cleanEmoji(strings.TrimPrefix(trimmed, "## "))
			if !firstSection {
				y += 10
			}
			dc.SetLineWidth(1)
			dc.SetColor(color.Black)
			dc.DrawLine(margin, y, margin+contentW, y)
			dc.Stroke()
			y += 35
			dc.SetFontFace(sectionFace)
			dc.DrawString(strings.ToUpper(text), margin, y)
			y += 45
			firstSection = false

		case strings.HasPrefix(trimmed, "> "):
			text := cleanEmoji(strings.TrimPrefix(trimmed, "> "))
			dc.SetFontFace(quoteFace)
			dc.SetColor(color.Gray{Y: 100})
			dc.DrawString(text, margin+20, y)
			dc.SetColor(color.Black)
			y += 35

		case strings.HasPrefix(trimmed, "- "):
			text := strings.TrimPrefix(trimmed, "- ")
			spans := parseInline(text)
			dc.SetFontFace(bodyFace)
			dc.SetColor(color.Black)
			dc.DrawString("\u2022 ", margin, y)
			bw, _ := dc.MeasureString("\u2022 ")
			drawSpans(dc, spans, margin+bw, y)
			y += 35

		case len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed, ". "):
			idx := strings.Index(trimmed, ". ")
			num := trimmed[:idx+1]
			text := trimmed[idx+2:]
			spans := parseInline(text)
			dc.SetFontFace(bodyFace)
			dc.SetColor(color.Black)
			dc.DrawString(num+" ", margin, y)
			nw, _ := dc.MeasureString(num + " ")
			drawSpans(dc, spans, margin+nw, y)
			y += 35

		default:
			spans := parseInline(trimmed)
			dc.SetColor(color.Black)
			drawSpans(dc, spans, margin, y)
			y += 35
		}
	}

	return dc.Image()
}

func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return gray
}

func floydSteinberg(gray *image.Gray) *image.Gray {
	bounds := gray.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	errors := make([][]float64, h)
	for i := range errors {
		errors[i] = make([]float64, w)
	}

	out := image.NewGray(bounds)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			oldVal := float64(gray.GrayAt(x+bounds.Min.X, y+bounds.Min.Y).Y) + errors[y][x]
			var newVal float64
			if oldVal > 127 {
				newVal = 255
			}
			out.SetGray(x+bounds.Min.X, y+bounds.Min.Y, color.Gray{Y: uint8(newVal)})

			err := oldVal - newVal
			if x+1 < w {
				errors[y][x+1] += err * 7 / 16
			}
			if y+1 < h {
				if x-1 >= 0 {
					errors[y+1][x-1] += err * 3 / 16
				}
				errors[y+1][x] += err * 5 / 16
				if x+1 < w {
					errors[y+1][x+1] += err * 1 / 16
				}
			}
		}
	}
	return out
}
