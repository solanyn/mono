package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	mxcfbSendUpdate = 0x4048462e
)

type mxcfbUpdateData struct {
	top          uint32
	left         uint32
	width        uint32
	height       uint32
	waveformMode uint32
	updateMode   uint32
	updateMarker uint32
	temp         int32
	flags        uint32
	ditherMode   int32
	quantBit     int32
	_            [1]int32
}

var (
	serverURL string
	interval  time.Duration
	fbDev     string
	noUpdate  bool
)

func init() {
	flag.StringVar(&serverURL, "url", envOr("DASH_URL", "http://10.0.0.1:8080"), "dashboard server base URL")
	flag.DurationVar(&interval, "interval", envOrDuration("DASH_INTERVAL", time.Hour), "refresh interval")
	flag.StringVar(&fbDev, "fb", envOr("DASH_FB", "/dev/fb0"), "framebuffer device")
	flag.BoolVar(&noUpdate, "no-update", false, "disable self-update")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	log.Printf("kobo-dash: url=%s interval=%s fb=%s no-update=%v", serverURL, interval, fbDev, noUpdate)

	for {
		if err := refresh(ctx); err != nil {
			log.Printf("refresh error: %v", err)
		}

		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		case <-time.After(interval):
		}
	}
}

func refresh(ctx context.Context) error {
	wifiOn()
	defer wifiOff()

	time.Sleep(2 * time.Second)

	if !noUpdate {
		if err := checkUpdate(ctx); err != nil {
			log.Printf("update check: %v", err)
		}
	}

	data, err := fetchPNG(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	if err := writeFramebuffer(data); err != nil {
		return fmt.Errorf("fb write: %w", err)
	}

	if err := triggerRefresh(); err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	return nil
}

func checkUpdate(ctx context.Context) error {
	remoteHash, err := fetchText(ctx, serverURL+"/kobo-dash.sha256")
	if err != nil {
		return fmt.Errorf("fetch remote hash: %w", err)
	}
	remoteHash = strings.TrimSpace(remoteHash)

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find self: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve self: %w", err)
	}

	localHash, err := hashFile(selfPath)
	if err != nil {
		return fmt.Errorf("hash self: %w", err)
	}

	if localHash == remoteHash {
		return nil
	}

	log.Printf("update available: local=%s remote=%s", localHash[:12], remoteHash[:12])

	binData, err := fetchBytes(ctx, serverURL+"/kobo-dash.bin")
	if err != nil {
		return fmt.Errorf("fetch binary: %w", err)
	}

	h := sha256.Sum256(binData)
	dlHash := hex.EncodeToString(h[:])
	if dlHash != remoteHash {
		return fmt.Errorf("hash mismatch: expected %s got %s", remoteHash[:12], dlHash[:12])
	}

	tmpPath := selfPath + ".tmp"
	if err := os.WriteFile(tmpPath, binData, 0755); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}

	if err := os.Rename(tmpPath, selfPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	log.Printf("updated, restarting")
	return syscall.Exec(selfPath, os.Args, os.Environ())
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

func fetchText(ctx context.Context, url string) (string, error) {
	data, err := fetchBytes(ctx, url)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fetchPNG(ctx context.Context) ([]byte, error) {
	return fetchBytes(ctx, serverURL+"/dashboard.png")
}

func writeFramebuffer(data []byte) error {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("png decode: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	pixels := make([]byte, w*h)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			gray := color.GrayModel.Convert(color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: 255,
			}).(color.Gray)
			pixels[y*w+x] = gray.Y
		}
	}

	return os.WriteFile(fbDev, pixels, 0644)
}

func triggerRefresh() error {
	fb, err := os.OpenFile(fbDev, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer fb.Close()

	update := mxcfbUpdateData{
		waveformMode: 2, // GC16
		updateMode:   1, // full
		updateMarker: 1,
		temp:         0x1001, // TEMP_USE_AMBIENT
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fb.Fd(),
		mxcfbSendUpdate,
		uintptr(unsafe.Pointer(&update)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func wifiOn() {
	exec.Command("/usr/bin/wifi", "on").Run()
}

func wifiOff() {
	exec.Command("/usr/bin/wifi", "off").Run()
}
