package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	flag.StringVar(&binDir, "bindir", envOr("DASH_BINDIR", "/app"), "directory containing kobo-dash binary")
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
	boldRe    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	emojiRe   = regexp.MustCompile(`[\x{1F000}-\x{1FFFF}]|[\x{2600}-\x{27BF}]|[\x{FE00}-\x{FEFF}]|[\x{1F900}-\x{1F9FF}]|[\x{200D}\x{20E3}\x{FE0F}]`)
	warnRe    = regexp.MustCompile(`\x{26A0}\x{FE0F}?`)
	numListRe = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/dashboard.png", handleDashboard)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/kobo-dash.bin", handleBinary)
	mux.HandleFunc("/kobo-dash.sha256", handleBinarySHA256)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		log.Printf("listening on %s (%dx%d) file=%s bindir=%s", addr, width, height, mdFile, binDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
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

	f, err := os.Open(binPath())
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	binHashCache = hex.EncodeToString(h.Sum(nil))
	binHashModTime = info.ModTime()
	return binHashCache, nil
}

func handleBinary(w http.ResponseWriter, r *http.Request) {
	hash, err := computeBinHash()
	if err == nil {
		etag := `"` + hash + `"`
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFile(w, r, binPath())
}

func handleBinarySHA256(w http.ResponseWriter, r *http.Request) {
	hash, err := computeBinHash()
	if err != nil {
		http.Error(w, "binary not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintln(w, hash)
}

type dashCache struct {
	mu      sync.Mutex
	modTime time.Time
	size    int64
	png     []byte
	etag    string
}

var dashboardCache dashCache

func renderDashboard() ([]byte, string, error) {
	dashboardCache.mu.Lock()
	defer dashboardCache.mu.Unlock()

	info, err := os.Stat(mdFile)
	var modTime time.Time
	var size int64
	if err == nil {
		modTime = info.ModTime()
		size = info.Size()
	}

	if dashboardCache.png != nil &&
		dashboardCache.modTime.Equal(modTime) &&
		dashboardCache.size == size {
		return dashboardCache.png, dashboardCache.etag, nil
	}

	var content []byte
	if err == nil {
		content, err = os.ReadFile(mdFile)
		if err != nil {
			log.Printf("read %s: %v", mdFile, err)
			content = nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Printf("stat %s: %v", mdFile, err)
	}

	img := renderMarkdown(width, height, content)
	gray := toGrayscale(img)
	dithered := floydSteinberg(gray)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dithered); err != nil {
		return nil, "", err
	}

	sum := sha256.Sum256(buf.Bytes())
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	dashboardCache.png = buf.Bytes()
	dashboardCache.etag = etag
	dashboardCache.modTime = modTime
	dashboardCache.size = size

	return dashboardCache.png, etag, nil
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	data, etag, err := renderDashboard()
	if err != nil {
		log.Printf("render: %v", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")

	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if _, err := w.Write(data); err != nil {
		log.Printf("write response: %v", err)
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
	truncated := false

	for _, line := range lines {
		if y > float64(h)-40 {
			truncated = true
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

		default:
			if m := numListRe.FindStringSubmatch(trimmed); m != nil {
				num := m[1] + "."
				text := m[2]
				spans := parseInline(text)
				dc.SetFontFace(bodyFace)
				dc.SetColor(color.Black)
				dc.DrawString(num+" ", margin, y)
				nw, _ := dc.MeasureString(num + " ")
				drawSpans(dc, spans, margin+nw, y)
				y += 35
				continue
			}
			spans := parseInline(trimmed)
			dc.SetColor(color.Black)
			drawSpans(dc, spans, margin, y)
			y += 35
		}
	}

	if truncated {
		dc.SetFontFace(bodyFace)
		dc.SetColor(color.Gray{Y: 100})
		dc.DrawStringAnchored("\u2026", float64(w)/2, float64(h)-20, 0.5, 0.5)
	}

	return dc.Image()
}

func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	if g, ok := img.(*image.Gray); ok {
		return g
	}
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

	curr := make([]int32, w+2)
	next := make([]int32, w+2)

	out := image.NewGray(bounds)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			oldVal := int32(gray.Pix[y*gray.Stride+x]) + curr[x+1]
			var newVal int32
			if oldVal > 127 {
				newVal = 255
			}
			out.Pix[y*out.Stride+x] = uint8(newVal)

			qerr := oldVal - newVal
			curr[x+2] += qerr * 7 / 16
			next[x] += qerr * 3 / 16
			next[x+1] += qerr * 5 / 16
			next[x+2] += qerr * 1 / 16
		}
		curr, next = next, curr
		for i := range next {
			next[i] = 0
		}
	}
	return out
}
