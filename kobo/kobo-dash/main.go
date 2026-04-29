package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	fbioGetVScreenInfo = 0x4600
	fbioGetFScreenInfo = 0x4602

	mxcfbSendUpdate = 0x4048462e
)

type fbBitfield struct {
	offset   uint32
	length   uint32
	msbRight uint32
}

type fbVarScreenInfo struct {
	xres, yres                 uint32
	xresVirtual, yresVirtual   uint32
	xoffset, yoffset           uint32
	bitsPerPixel               uint32
	grayscale                  uint32
	red, green, blue, transp   fbBitfield
	nonstd                     uint32
	activate                   uint32
	height, width              uint32
	accelFlags                 uint32
	pixclock                   uint32
	leftMargin, rightMargin    uint32
	upperMargin, lowerMargin   uint32
	hsyncLen, vsyncLen         uint32
	sync                       uint32
	vmode                      uint32
	rotate                     uint32
	colorspace                 uint32
	reserved                   [4]uint32
}

type fbFixScreenInfo struct {
	id           [16]byte
	smemStart    uint64
	smemLen      uint32
	fbType       uint32
	typeAux      uint32
	visual       uint32
	xpanstep     uint16
	ypanstep     uint16
	ywrapstep    uint16
	_pad         uint16
	lineLength   uint32
	mmioStart    uint64
	mmioLen      uint32
	accel        uint32
	capabilities uint16
	reserved     [2]uint16
}

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
	serverURL  string
	interval   time.Duration
	fbDev      string
	noUpdate   bool
	logFile    string
	probeMode  bool
	testMode   bool
	httpClient = &http.Client{Timeout: 30 * time.Second}
)

var (
	lastDashETag   string
	updateMarkerID uint32 = 1
)

func init() {
	flag.StringVar(&serverURL, "url", envOr("DASH_URL", "http://10.0.0.1:8080"), "dashboard server base URL")
	flag.DurationVar(&interval, "interval", envOrDuration("DASH_INTERVAL", time.Hour), "refresh interval")
	flag.StringVar(&fbDev, "fb", envOr("DASH_FB", "/dev/fb0"), "framebuffer device")
	flag.BoolVar(&noUpdate, "no-update", false, "disable self-update")
	flag.StringVar(&logFile, "log", envOr("DASH_LOG", "/mnt/onboard/.adds/kobo-dash/kobo-dash.log"), "log file path (empty to disable)")
	flag.BoolVar(&probeMode, "probe", false, "probe framebuffer and exit")
	flag.BoolVar(&testMode, "test", false, "write test gradient to framebuffer and exit")
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

func setupLogging() func() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if logFile == "" {
		return func() {}
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		log.Printf("log mkdir: %v", err)
		return func() {}
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("log open %s: %v", logFile, err)
		return func() {}
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return func() { f.Close() }
}

func main() {
	flag.Parse()
	closeLog := setupLogging()
	defer closeLog()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v\n%s", r, debug.Stack())
			os.Exit(2)
		}
	}()

	log.Printf("kobo-dash start: url=%s interval=%s fb=%s no-update=%v probe=%v test=%v",
		serverURL, interval, fbDev, noUpdate, probeMode, testMode)

	if probeMode {
		if err := probe(); err != nil {
			log.Printf("probe error: %v", err)
			os.Exit(1)
		}
		return
	}

	if testMode {
		if err := runTest(); err != nil {
			log.Printf("test error: %v", err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

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

func probe() error {
	fb, err := os.OpenFile(fbDev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", fbDev, err)
	}
	defer fb.Close()

	var vinfo fbVarScreenInfo
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fb.Fd(),
		fbioGetVScreenInfo, uintptr(unsafe.Pointer(&vinfo))); errno != 0 {
		return fmt.Errorf("FBIOGET_VSCREENINFO: %w", errno)
	}

	var finfo fbFixScreenInfo
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fb.Fd(),
		fbioGetFScreenInfo, uintptr(unsafe.Pointer(&finfo))); errno != 0 {
		return fmt.Errorf("FBIOGET_FSCREENINFO: %w", errno)
	}

	id := string(bytes.TrimRight(finfo.id[:], "\x00"))
	log.Printf("fb id:            %q", id)
	log.Printf("fb resolution:    %dx%d (virtual %dx%d)",
		vinfo.xres, vinfo.yres, vinfo.xresVirtual, vinfo.yresVirtual)
	log.Printf("fb bits/pixel:    %d", vinfo.bitsPerPixel)
	log.Printf("fb grayscale:     %d", vinfo.grayscale)
	log.Printf("fb rotate:        %d", vinfo.rotate)
	log.Printf("fb line_length:   %d", finfo.lineLength)
	log.Printf("fb smem_len:      %d", finfo.smemLen)
	log.Printf("fb red:           offset=%d length=%d", vinfo.red.offset, vinfo.red.length)
	log.Printf("fb green:         offset=%d length=%d", vinfo.green.offset, vinfo.green.length)
	log.Printf("fb blue:          offset=%d length=%d", vinfo.blue.offset, vinfo.blue.length)

	expected := vinfo.yres * finfo.lineLength
	log.Printf("fb expected write size: %d bytes (yres * line_length)", expected)

	return nil
}

type fbInfo struct {
	xres, yres   int
	bpp          int
	lineLength   int
	grayscale    uint32
	redOff, redLen uint32
	id           string
}

func readFBInfo(fb *os.File) (fbInfo, error) {
	var vinfo fbVarScreenInfo
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fb.Fd(),
		fbioGetVScreenInfo, uintptr(unsafe.Pointer(&vinfo))); errno != 0 {
		return fbInfo{}, fmt.Errorf("FBIOGET_VSCREENINFO: %w", errno)
	}
	var finfo fbFixScreenInfo
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fb.Fd(),
		fbioGetFScreenInfo, uintptr(unsafe.Pointer(&finfo))); errno != 0 {
		return fbInfo{}, fmt.Errorf("FBIOGET_FSCREENINFO: %w", errno)
	}
	return fbInfo{
		xres:       int(vinfo.xres),
		yres:       int(vinfo.yres),
		bpp:        int(vinfo.bitsPerPixel),
		lineLength: int(finfo.lineLength),
		grayscale:  vinfo.grayscale,
		redOff:     vinfo.red.offset,
		redLen:     vinfo.red.length,
		id:         string(bytes.TrimRight(finfo.id[:], "\x00")),
	}, nil
}

func runTest() error {
	fb, err := os.OpenFile(fbDev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer fb.Close()

	info, err := readFBInfo(fb)
	if err != nil {
		return err
	}
	log.Printf("test: fb %s %dx%d bpp=%d stride=%d",
		info.id, info.xres, info.yres, info.bpp, info.lineLength)

	buf := make([]byte, info.yres*info.lineLength)
	for y := 0; y < info.yres; y++ {
		gray := uint8((y * 255) / info.yres)
		row := buf[y*info.lineLength : y*info.lineLength+info.lineLength]
		packRow(row, gray, info)
	}
	if _, err := fb.WriteAt(buf, 0); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	log.Printf("test: wrote gradient (%d bytes)", len(buf))

	if err := triggerRefresh(fb); err != nil {
		log.Printf("test: refresh ioctl failed: %v (screen may still update on next native refresh)", err)
	} else {
		log.Printf("test: refresh ioctl ok")
	}
	return nil
}

func packRow(row []byte, gray uint8, info fbInfo) {
	switch info.bpp {
	case 8:
		for i := range row {
			row[i] = gray
		}
	case 16:
		// rgb565
		r5 := uint16(gray>>3) & 0x1f
		g6 := uint16(gray>>2) & 0x3f
		b5 := uint16(gray>>3) & 0x1f
		px := (r5 << 11) | (g6 << 5) | b5
		for i := 0; i+1 < len(row); i += 2 {
			row[i] = byte(px)
			row[i+1] = byte(px >> 8)
		}
	case 32:
		for i := 0; i+3 < len(row); i += 4 {
			row[i+0] = gray
			row[i+1] = gray
			row[i+2] = gray
			row[i+3] = 0xff
		}
	}
}

func refresh(ctx context.Context) error {
	wifiOn()
	defer wifiOff()

	if err := waitForNetwork(ctx, 20*time.Second); err != nil {
		return fmt.Errorf("wifi: %w", err)
	}

	if !noUpdate {
		if err := checkUpdate(ctx); err != nil {
			log.Printf("update check: %v", err)
		}
	}

	data, etag, notModified, err := fetchDashboard(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if notModified {
		log.Printf("dashboard unchanged (etag=%s)", truncate(etag, 12))
		return nil
	}

	if err := writeDashboard(data); err != nil {
		return fmt.Errorf("fb write: %w", err)
	}

	lastDashETag = etag
	return nil
}

func waitForNetwork(ctx context.Context, max time.Duration) error {
	deadline := time.Now().Add(max)
	url := serverURL + "/health"
	for {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		resp, err := httpClient.Do(req)
		cancel()
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("health check failed after %s", max)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
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

	log.Printf("update available: local=%s remote=%s", truncate(localHash, 12), truncate(remoteHash, 12))

	binData, err := fetchBytes(ctx, serverURL+"/kobo-dash.bin")
	if err != nil {
		return fmt.Errorf("fetch binary: %w", err)
	}

	h := sha256.Sum256(binData)
	dlHash := hex.EncodeToString(h[:])
	if dlHash != remoteHash {
		return fmt.Errorf("hash mismatch: expected %s got %s", truncate(remoteHash, 12), truncate(dlHash, 12))
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
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fetchDashboard(ctx context.Context) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/dashboard.png", nil)
	if err != nil {
		return nil, "", false, err
	}
	if lastDashETag != "" {
		req.Header.Set("If-None-Match", lastDashETag)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()

	etag := resp.Header.Get("ETag")

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, etag, true, nil
	case http.StatusOK:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", false, err
		}
		return data, etag, false, nil
	default:
		return nil, "", false, fmt.Errorf("status %d", resp.StatusCode)
	}
}

func writeDashboard(data []byte) error {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("png decode: %w", err)
	}

	fb, err := os.OpenFile(fbDev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open fb: %w", err)
	}
	defer fb.Close()

	info, err := readFBInfo(fb)
	if err != nil {
		return err
	}

	gray := toGray(img)
	bounds := gray.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	if imgW != info.xres || imgH != info.yres {
		log.Printf("warn: image %dx%d != fb %dx%d", imgW, imgH, info.xres, info.yres)
	}

	buf := make([]byte, info.yres*info.lineLength)
	h := imgH
	if h > info.yres {
		h = info.yres
	}
	w := imgW
	if w > info.xres {
		w = info.xres
	}

	for y := 0; y < h; y++ {
		row := buf[y*info.lineLength : y*info.lineLength+info.lineLength]
		srcRow := gray.Pix[y*gray.Stride : y*gray.Stride+w]
		packGrayRow(row, srcRow, info)
	}

	if _, err := fb.WriteAt(buf, 0); err != nil {
		return fmt.Errorf("fb write: %w", err)
	}

	if err := triggerRefresh(fb); err != nil {
		log.Printf("refresh ioctl failed: %v", err)
	}
	return nil
}

func toGray(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	b := img.Bounds()
	g := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g.Set(x, y, img.At(x, y))
		}
	}
	return g
}

func packGrayRow(dst, src []byte, info fbInfo) {
	switch info.bpp {
	case 8:
		copy(dst, src)
	case 16:
		for x := 0; x < len(src) && (x*2+1) < len(dst); x++ {
			v := src[x]
			r5 := uint16(v>>3) & 0x1f
			g6 := uint16(v>>2) & 0x3f
			b5 := uint16(v>>3) & 0x1f
			px := (r5 << 11) | (g6 << 5) | b5
			dst[x*2] = byte(px)
			dst[x*2+1] = byte(px >> 8)
		}
	case 32:
		for x := 0; x < len(src) && (x*4+3) < len(dst); x++ {
			v := src[x]
			dst[x*4+0] = v
			dst[x*4+1] = v
			dst[x*4+2] = v
			dst[x*4+3] = 0xff
		}
	default:
		// fallback: best-effort write raw bytes
		n := copy(dst, src)
		_ = n
	}
	_ = color.Gray{}
}

func triggerRefresh(fb *os.File) error {
	updateMarkerID++
	if updateMarkerID == 0 {
		updateMarkerID = 1
	}

	update := mxcfbUpdateData{
		waveformMode: 2, // GC16
		updateMode:   1, // full
		updateMarker: updateMarkerID,
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
	if err := exec.Command("/usr/bin/wifi", "on").Run(); err != nil {
		log.Printf("wifi on: %v", err)
	}
}

func wifiOff() {
	if err := exec.Command("/usr/bin/wifi", "off").Run(); err != nil {
		log.Printf("wifi off: %v", err)
	}
}
