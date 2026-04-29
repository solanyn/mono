package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/solanyn/mono/scrib/audio"
	"github.com/solanyn/mono/scrib/client"
)

type phase int

const (
	phaseRecording phase = iota
	phaseProcessing
	phaseFailed
	phaseResults
)

type Options struct {
	Name       string
	OutputPath string
	SampleRate int
	ServerURL  string
	Template   string
	OutboxDir  string // optional; if set, phaseFailed 's' key parks the upload here
}

// ProgressEvent mirrors client.Progress so the TUI doesn't have to import the
// client package (avoids a circular dep when the uploader reuses the TUI).
type ProgressEvent struct {
	Stage   string
	Attempt int
	Err     error
}

// UploadFunc runs the full metadata → audio → process upload, sending a
// ProgressEvent after every attempt. It must honour ctx cancellation.
type UploadFunc func(ctx context.Context, samples []int16, opts Options, progress func(ProgressEvent)) (string, error)

type model struct {
	opts      Options
	recorder  *audio.Recorder
	phase     phase
	width     int
	height    int
	startTime time.Time
	elapsed   time.Duration
	uploadFn  UploadFunc

	spinner  spinner.Model
	steps    []processStep
	stepIdx  int
	attempt  int
	detail   string
	monoPath string
	samples  []int16

	result   *pipelineResult
	resultVP viewport.Model

	uploadCancel context.CancelFunc
	err          error
}

type pipelineResult struct {
	UUID     string
	Duration time.Duration
}

type processStep struct {
	key   string // "save" | "metadata" | "audio" | "process"
	label string
	done  bool
}

type tickMsg time.Time
type pipelineDoneMsg struct {
	result *pipelineResult
	err    error
}
type pipelineProgressMsg ProgressEvent
type savedMsg struct {
	monoPath string
	samples  []int16
	dur      time.Duration
	err      error
}

// uploadReadyMsg fires once the client upload has successfully POSTed
// /v1/process — from here on progress comes from the server SSE stream,
// not the uploader's own retry loop.
type uploadReadyMsg struct {
	uuid string
	dur  time.Duration
}

// sseStageMsg carries a single decoded server-sent event.
type sseStageMsg struct {
	stage string
	data  []byte
}

// sseDoneMsg signals the SSE subscription terminated.
type sseDoneMsg struct {
	err error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	meterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	waveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func buildSteps(hasUpload bool) []processStep {
	steps := []processStep{{key: "save", label: "Saving audio"}}
	if hasUpload {
		steps = append(steps,
			processStep{key: "metadata", label: "Creating meeting"},
			processStep{key: "audio", label: "Uploading audio"},
			processStep{key: "process", label: "Requesting processing"},
			processStep{key: "processing", label: "Transcribing + diarizing"},
			processStep{key: "matching", label: "Matching speakers"},
		)
	}
	return steps
}

func initialModel(opts Options, uploadFn UploadFunc) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		opts:     opts,
		phase:    phaseRecording,
		spinner:  sp,
		uploadFn: uploadFn,
		steps:    buildSteps(uploadFn != nil && opts.ServerURL != ""),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resultVP = viewport.New(viewport.WithWidth(max(m.width-4, 1)), viewport.WithHeight(max(m.height-4, 1)))

	case tickMsg:
		if m.phase == phaseRecording {
			m.elapsed = time.Since(m.startTime)
		}
		cmds = append(cmds, tickCmd())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case savedMsg:
		if msg.err != nil {
			m.phase = phaseFailed
			m.err = msg.err
			return m, nil
		}
		// Mark "save" done and kick off the upload.
		m.markStepDone("save")
		m.monoPath = msg.monoPath
		m.samples = nil // released, on-disk copy from here on
		if m.uploadFn == nil || m.opts.ServerURL == "" {
			m.result = &pipelineResult{Duration: msg.dur}
			m.phase = phaseResults
			m.resultVP = viewport.New(viewport.WithWidth(max(m.width-4, 1)), viewport.WithHeight(max(m.height-4, 1)))
			m.resultVP.SetContent(m.renderResults())
			return m, nil
		}
		m.advanceTo("metadata")
		cmd, cancel := m.startUpload(msg.samples, msg.dur)
		m.uploadCancel = cancel
		cmds = append(cmds, cmd)

	case pipelineProgressMsg:
		m.attempt = msg.Attempt
		if msg.Err != nil {
			m.detail = fmt.Sprintf("%s attempt %d failed: %v", msg.Stage, msg.Attempt, msg.Err)
		} else {
			m.markStepDone(msg.Stage)
			m.detail = ""
			m.advanceAfter(msg.Stage)
		}

	case uploadReadyMsg:
		// Client-side upload completed; hand off to SSE for server-side stages.
		m.result = &pipelineResult{UUID: msg.uuid, Duration: msg.dur}
		m.advanceTo("processing")
		m.detail = ""
		cmd, cancel := m.startSSE(msg.uuid)
		m.uploadCancel = cancel
		cmds = append(cmds, cmd)

	case sseStageMsg:
		// Server emits: started, processing, matching, done, error.
		switch msg.stage {
		case "started", "processing":
			m.advanceTo("processing")
			m.detail = ""
		case "matching":
			m.markStepDone("processing")
			m.advanceTo("matching")
			m.detail = ""
		case "done":
			m.markStepDone("processing")
			m.markStepDone("matching")
		case "error":
			if m.err == nil {
				m.err = fmt.Errorf("server: %s", string(msg.data))
			}
		}

	case sseDoneMsg:
		m.uploadCancel = nil
		if msg.err != nil && m.err == nil {
			m.err = msg.err
		}
		if m.err != nil {
			m.phase = phaseFailed
			return m, nil
		}
		m.phase = phaseResults
		m.resultVP = viewport.New(viewport.WithWidth(max(m.width-4, 1)), viewport.WithHeight(max(m.height-4, 1)))
		m.resultVP.SetContent(m.renderResults())

	case pipelineDoneMsg:
		m.uploadCancel = nil
		if msg.err != nil {
			m.err = msg.err
			m.phase = phaseFailed
			return m, nil
		}
		m.result = msg.result
		m.phase = phaseResults
		m.resultVP = viewport.New(viewport.WithWidth(max(m.width-4, 1)), viewport.WithHeight(max(m.height-4, 1)))
		m.resultVP.SetContent(m.renderResults())
	}

	if m.phase == phaseResults {
		var cmd tea.Cmd
		m.resultVP, cmd = m.resultVP.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) markStepDone(key string) {
	for i := range m.steps {
		if m.steps[i].key == key {
			m.steps[i].done = true
			return
		}
	}
}

func (m *model) advanceTo(key string) {
	for i, s := range m.steps {
		if s.key == key {
			m.stepIdx = i
			return
		}
	}
}

func (m *model) advanceAfter(key string) {
	for i, s := range m.steps {
		if s.key == key && i+1 < len(m.steps) {
			m.stepIdx = i + 1
			return
		}
	}
}

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.phase {
	case phaseRecording:
		if key == "ctrl+c" {
			m.phase = phaseProcessing
			m.stepIdx = 0
			return m, m.runSave()
		}

	case phaseProcessing:
		if key == "ctrl+c" && m.uploadCancel != nil {
			m.uploadCancel()
			m.uploadCancel = nil
			m.phase = phaseFailed
			m.err = fmt.Errorf("cancelled")
			return m, nil
		}

	case phaseFailed:
		switch key {
		case "r":
			// Retry upload only — audio is already saved.
			if m.monoPath != "" {
				m.phase = phaseProcessing
				m.err = nil
				m.detail = ""
				m.resetStepState()
				m.markStepDone("save")
				m.advanceTo("metadata")
				cmd, cancel := m.startUploadFromPath(m.monoPath)
				m.uploadCancel = cancel
				return m, cmd
			}
		case "s":
			// Save-and-quit: audio is already on disk at m.opts.OutputPath.
			// If an outbox is configured, also park a retry record so
			// `scrib upload` or startup-resume can pick it up later.
			if m.opts.OutboxDir != "" && m.monoPath != "" {
				_, _ = client.OutboxWrite(m.opts.OutboxDir, client.OutboxEntry{
					Name:      m.opts.Name,
					Template:  m.opts.Template,
					WAVPath:   m.monoPath,
					ServerURL: m.opts.ServerURL,
					LastErr:   errString(m.err),
				})
			}
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case phaseResults:
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *model) resetStepState() {
	for i := range m.steps {
		if m.steps[i].key != "save" {
			m.steps[i].done = false
		}
	}
	m.stepIdx = 0
	m.attempt = 0
}

func (m model) runSave() tea.Cmd {
	return func() tea.Msg {
		samples := m.recorder.Stop()
		if err := audio.WriteWAV(m.opts.OutputPath, samples, m.opts.SampleRate, 2); err != nil {
			return savedMsg{err: err}
		}
		dur := time.Duration(len(samples)/2/m.opts.SampleRate) * time.Second

		// Pre-compute mono WAV to disk so retry doesn't re-downmix and doesn't
		// hold the full stereo buffer in RAM across the upload.
		mono := audio.StereoToMono(samples)
		monoPath, err := audio.WriteWAVTemp(mono, m.opts.SampleRate, 1)
		if err != nil {
			return savedMsg{err: fmt.Errorf("mono wav: %w", err)}
		}
		return savedMsg{monoPath: monoPath, samples: mono, dur: dur}
	}
}

func (m *model) startUpload(samples []int16, dur time.Duration) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	fn := m.uploadFn
	opts := m.opts
	progressCh := make(chan ProgressEvent, 8)
	uuidCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(progressCh)
		uuid, err := fn(ctx, samples, opts, func(e ProgressEvent) {
			select {
			case progressCh <- e:
			case <-ctx.Done():
			}
		})
		if err != nil {
			errCh <- err
			return
		}
		uuidCh <- uuid
	}()

	return m.waitUpload(ctx, progressCh, uuidCh, errCh, dur), cancel
}

func (m *model) startUploadFromPath(monoPath string) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	fn := m.uploadFn
	opts := m.opts
	progressCh := make(chan ProgressEvent, 8)
	uuidCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Load samples back from disk for the uploader. Keeps the API symmetrical
	// even though the server only cares about the on-disk WAV.
	samples, err := audio.ReadWAVMono(monoPath)
	if err != nil {
		cancel()
		close(progressCh)
		return func() tea.Msg { return pipelineDoneMsg{err: err} }, func() {}
	}
	dur := time.Duration(len(samples)/opts.SampleRate) * time.Second

	go func() {
		defer close(progressCh)
		uuid, err := fn(ctx, samples, opts, func(e ProgressEvent) {
			select {
			case progressCh <- e:
			case <-ctx.Done():
			}
		})
		if err != nil {
			errCh <- err
			return
		}
		uuidCh <- uuid
	}()

	return m.waitUpload(ctx, progressCh, uuidCh, errCh, dur), cancel
}

// waitUpload bridges the uploader goroutine to the tea message stream. Each
// call returns ONE message; the caller chains further waits via tea.Batch.
func (m *model) waitUpload(ctx context.Context, progressCh <-chan ProgressEvent, uuidCh <-chan string, errCh <-chan error, dur time.Duration) tea.Cmd {
	// Collect on a separate goroutine so the tea loop gets a steady stream
	// of progress + the final done message.
	resultCh := make(chan tea.Msg, cap(progressCh)+2)
	go func() {
		defer close(resultCh)
		for ev := range progressCh {
			resultCh <- pipelineProgressMsg(ev)
		}
		// Progress channel closed — uploader returned. Surface its result.
		select {
		case uuid, ok := <-uuidCh:
			if ok && uuid != "" {
				resultCh <- uploadReadyMsg{uuid: uuid, dur: dur}
				return
			}
		default:
		}
		select {
		case err := <-errCh:
			resultCh <- pipelineDoneMsg{err: err}
		default:
			resultCh <- pipelineDoneMsg{err: fmt.Errorf("upload ended without success")}
		}
	}()
	return func() tea.Msg {
		msg, ok := <-resultCh
		if !ok {
			return pipelineDoneMsg{err: ctx.Err()}
		}
		return msg
	}
}

// startSSE subscribes to /v1/process/{uuid}/events and feeds stage + done
// messages back into the tea loop. Returns the Cmd + a cancel func so
// ctrl+c in the processing phase can tear the stream down.
func (m *model) startSSE(uuid string) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	serverURL := m.opts.ServerURL
	events := make(chan sseStageMsg, 16)
	doneCh := make(chan error, 1)

	go func() {
		defer close(events)
		err := client.StreamProcess(ctx, serverURL, uuid, func(e client.StreamEvent) {
			select {
			case events <- sseStageMsg{stage: e.Stage, data: e.Data}:
			case <-ctx.Done():
			}
		})
		if client.IsStreamUnavailable(err) {
			// Processing already done / not streaming — treat as success;
			// the server will have its state reflected in /v1/meetings.
			doneCh <- nil
			return
		}
		doneCh <- err
	}()

	return m.waitSSE(events, doneCh), cancel
}

func (m *model) waitSSE(events <-chan sseStageMsg, doneCh <-chan error) tea.Cmd {
	out := make(chan tea.Msg, cap(events)+1)
	go func() {
		defer close(out)
		for ev := range events {
			out <- ev
		}
		err := <-doneCh
		out <- sseDoneMsg{err: err}
	}()
	return func() tea.Msg {
		msg, ok := <-out
		if !ok {
			return sseDoneMsg{}
		}
		return msg
	}
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	switch m.phase {
	case phaseRecording:
		v.SetContent(m.viewRecording())
	case phaseProcessing:
		v.SetContent(m.viewProcessing())
	case phaseFailed:
		v.SetContent(m.viewFailed())
	case phaseResults:
		v.SetContent(m.viewResults())
	}
	return v
}

func (m model) viewRecording() string {
	elapsed := fmt.Sprintf("%02d:%02d", int(m.elapsed.Minutes()), int(m.elapsed.Seconds())%60)
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))

	var statusParts []string
	statusParts = append(statusParts, dimStyle.Render(elapsed))

	if m.recorder != nil {
		micLvl := m.recorder.MicLevel()
		sysLvl := m.recorder.SysLevel()
		statusParts = append(statusParts, meterStyle.Render("mic")+renderMeter(micLvl))
		statusParts = append(statusParts, meterStyle.Render("sys")+renderMeter(sysLvl))
	}

	statusParts = append(statusParts, dimStyle.Render("ctrl+c: stop"))
	status := "  " + strings.Join(statusParts, dimStyle.Render("  │  "))

	var deviceInfo string
	captureStatus := audio.GetCaptureStatus()
	if m.recorder != nil {
		inputDev := audio.GetInputDeviceName()
		outputDev := audio.GetOutputDeviceName()
		deviceInfo = dimStyle.Render(fmt.Sprintf("  mic: %s  │  out: %s", inputDev, outputDev))
	}

	banner := ""
	switch captureStatus {
	case 1:
		banner = warnStyle.Render("  ⚠ system audio unavailable — recording mic only") + "\n"
	case -1:
		banner = errorStyle.Render("  ✗ capture failed") + "\n"
	}

	waveform := ""
	if m.recorder != nil {
		waveform = renderWaveform(m.recorder.MicLevel(), m.recorder.SysLevel(), max(m.width-6, 20))
	}

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-7, 1)).
		Render(banner + dimStyle.Render("\n  Recording... press ctrl+c to stop\n"))

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", title, content, waveform, status, deviceInfo)
}

func (m model) viewProcessing() string {
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))

	var sb strings.Builder
	sb.WriteString("\n  Processing...\n\n")
	for i, step := range m.steps {
		prefix := "    "
		body := dimStyle.Render(step.label)
		if step.done {
			prefix = "  "
			body = fmt.Sprintf("%s %s", checkStyle.Render("✓"), step.label)
		} else if i == m.stepIdx {
			prefix = "  "
			suffix := ""
			if m.attempt > 1 {
				suffix = dimStyle.Render(fmt.Sprintf("  (attempt %d)", m.attempt))
			}
			body = fmt.Sprintf("%s %s%s", m.spinner.View(), step.label, suffix)
		}
		sb.WriteString(prefix + body + "\n")
	}
	if m.detail != "" {
		sb.WriteString("\n  " + dimStyle.Render(m.detail) + "\n")
	}
	sb.WriteString("\n  " + dimStyle.Render("ctrl+c: cancel") + "\n")

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-4, 1)).
		Render(sb.String())

	return fmt.Sprintf("%s\n%s", title, content)
}

func (m model) viewFailed() string {
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))

	var sb strings.Builder
	sb.WriteString("\n  ")
	sb.WriteString(errorStyle.Render("✗ Upload failed"))
	sb.WriteString("\n\n")
	if m.err != nil {
		sb.WriteString("  " + m.err.Error() + "\n\n")
	}
	sb.WriteString("  Audio saved at: " + m.opts.OutputPath + "\n\n")
	sb.WriteString("  " + dimStyle.Render("r: retry   s: save and quit   q: quit") + "\n")

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-4, 1)).
		Render(sb.String())

	return fmt.Sprintf("%s\n%s", title, content)
}

func (m model) viewResults() string {
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))
	status := dimStyle.Render("  q: exit")

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-4, 1)).
		Render(m.resultVP.View())

	return fmt.Sprintf("%s\n%s\n%s", title, content, status)
}

func (m model) renderResults() string {
	if m.result == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Duration: %s\n", m.result.Duration.Round(time.Second)))
	sb.WriteString(fmt.Sprintf("  Audio: %s\n", m.opts.OutputPath))
	if m.result.UUID != "" {
		sb.WriteString(fmt.Sprintf("  Meeting: %s\n", m.result.UUID))
		sb.WriteString(fmt.Sprintf("\n  Processing on server. Use 'scrib status %s' to check.\n", m.result.UUID))
	}
	sb.WriteString("\n")
	return sb.String()
}

func renderMeter(level float64) string {
	const width = 8
	blocks := []string{"░", "▒", "▓", "█"}
	db := 20 * math.Log10(level+1e-10)
	norm := (db + 60) / 60
	if norm < 0 {
		norm = 0
	}
	if norm > 1 {
		norm = 1
	}
	filled := int(norm * width)
	var sb strings.Builder
	sb.WriteString(" ")
	for i := 0; i < width; i++ {
		if i < filled {
			idx := int(norm * float64(len(blocks)-1))
			sb.WriteString(meterStyle.Render(blocks[idx]))
		} else {
			sb.WriteString(dimStyle.Render("░"))
		}
	}
	return sb.String()
}

func renderWaveform(micLevel, sysLevel float64, width int) string {
	bars := []rune(" ▁▂▃▄▅▆▇")
	half := width / 2

	combined := (micLevel + sysLevel) / 2
	db := 20 * math.Log10(combined+1e-10)
	norm := (db + 60) / 60
	if norm < 0 {
		norm = 0
	}
	if norm > 1 {
		norm = 1
	}

	var sb strings.Builder
	sb.WriteString("  ")
	for i := 0; i < width; i++ {
		dist := float64(i-half) / float64(half)
		if dist < 0 {
			dist = -dist
		}
		amp := norm * (1 - dist*dist)
		if amp < 0 {
			amp = 0
		}
		idx := int(amp * float64(len(bars)-1))
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		sb.WriteRune(bars[idx])
	}
	return waveStyle.Render(sb.String())
}

func Run(opts Options, uploadFn UploadFunc) error {
	rec, err := audio.NewRecorder(opts.SampleRate)
	if err != nil {
		return fmt.Errorf("init recorder: %w", err)
	}

	if err := rec.Start(); err != nil {
		return fmt.Errorf("start recording: %w", err)
	}

	m := initialModel(opts, uploadFn)
	m.recorder = rec
	m.startTime = time.Now()

	// Ignore SIGINT at the process level so Ctrl+C doesn't kill us.
	// The terminal still sends the ctrl+c escape sequence through stdin,
	// which bubbletea reads as a KeyPressMsg and routes to handleKey.
	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt)

	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	fm := finalModel.(model)
	if fm.err != nil && fm.phase != phaseFailed {
		return fm.err
	}
	if fm.phase == phaseFailed {
		fmt.Fprintf(os.Stderr, "Upload failed: %v\n", fm.err)
		fmt.Fprintf(os.Stderr, "Audio saved at %s. Retry later with: scrib upload %s\n", opts.OutputPath, filepath.Base(opts.OutputPath))
	}
	if fm.phase == phaseResults {
		fmt.Printf("Saved: %s\n", opts.OutputPath)
	}

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
