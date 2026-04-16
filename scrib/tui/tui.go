package tui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/solanyn/mono/scrib/audio"
	"github.com/solanyn/mono/scrib/calendar"
	"github.com/solanyn/mono/scrib/client"
	"github.com/solanyn/mono/scrib/store"
)

type phase int

const (
	phaseLaunch phase = iota
	phaseRecording
	phaseConfirm
	phaseProcessing
	phaseResults
)

type Options struct {
	Name       string
	OutputPath string
	SampleRate int
	AudioURL   string
	GatewayURL string
	Client     *client.Client
	Template   string
	DB         *store.DB
	Events     []calendar.Event
}

type model struct {
	opts      Options
	recorder  *audio.Recorder
	phase     phase
	width     int
	height    int
	startTime time.Time
	elapsed   time.Duration

	// launch
	eventItems []eventItem
	cursor     int

	// recording
	transcript []transcriptLine
	viewport   viewport.Model
	lastChunk  time.Time

	// processing
	spinner spinner.Model
	steps   []processStep
	stepIdx int

	// results
	result   *client.AnnotateResult
	resultVP viewport.Model

	err error
}

type eventItem struct {
	event *calendar.Event // nil = ad hoc
	label string
}

type transcriptLine struct {
	ts   time.Duration
	text string
}

type processStep struct {
	label string
	done  bool
}

// messages
type tickMsg time.Time
type transcriptMsg struct {
	text string
	ts   time.Duration
}
type pipelineDoneMsg struct {
	result *client.AnnotateResult
	err    error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// styles
var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	selStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	nowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	meterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	waveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func initialModel(opts Options) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := model{
		opts:    opts,
		spinner: sp,
		steps: []processStep{
			{label: "Saving audio"},
			{label: "Speaker diarisation"},
			{label: "Transcription"},
			{label: "Summarising"},
		},
	}

	// Build event list
	nowIdx := -1
	for i, ev := range opts.Events {
		label := fmt.Sprintf("%s–%s  %s", ev.Start.Format("15:04"), ev.End.Format("15:04"), ev.Summary)
		if len(ev.Attendees) > 0 {
			label += fmt.Sprintf("  (%d attendees)", len(ev.Attendees))
		}
		item := eventItem{event: &opts.Events[i], label: label}
		m.eventItems = append(m.eventItems, item)
		if ev.IsNow() && nowIdx < 0 {
			nowIdx = i
		}
	}
	m.eventItems = append(m.eventItems, eventItem{label: "Ad hoc recording"})

	if nowIdx >= 0 {
		m.cursor = nowIdx
	}

	// Skip launch if no events
	if len(opts.Events) == 0 {
		m.phase = phaseRecording
	} else {
		m.phase = phaseLaunch
	}

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h := max(m.height-4, 1)
		w := max(m.width-4, 1)
		m.viewport = viewport.New(w, h)
		m.resultVP = viewport.New(w, h)

	case tickMsg:
		if m.phase == phaseRecording {
			m.elapsed = time.Since(m.startTime)
			// Live transcription every 10s
			if m.recorder != nil && time.Since(m.lastChunk) >= 10*time.Second && m.recorder.FrameCount() > 0 {
				m.lastChunk = time.Now()
				cmds = append(cmds, m.transcribeChunk())
			}
		}
		cmds = append(cmds, tickCmd())

	case transcriptMsg:
		if msg.text != "" {
			m.transcript = append(m.transcript, transcriptLine{ts: msg.ts, text: msg.text})
			m.viewport.SetContent(m.renderTranscript())
			m.viewport.GotoBottom()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case pipelineDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.result = msg.result
		m.phase = phaseResults
		m.resultVP = viewport.New(max(m.width-4, 1), max(m.height-4, 1))
		m.resultVP.SetContent(m.renderResults())
	}

	if m.phase == phaseRecording {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.phase == phaseResults {
		var cmd tea.Cmd
		m.resultVP, cmd = m.resultVP.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.phase {
	case phaseLaunch:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.eventItems)-1 {
				m.cursor++
			}
		case "enter":
			sel := m.eventItems[m.cursor]
			if sel.event != nil {
				m.opts.Name = sanitizeName(sel.event.Summary)
			}
			m.phase = phaseRecording
			m.startTime = time.Now()
			m.lastChunk = time.Now()
			return m, m.startRecording()
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case phaseRecording:
		if key == "ctrl+c" {
			m.phase = phaseConfirm
		}

	case phaseConfirm:
		switch key {
		case "p", "P":
			m.phase = phaseProcessing
			m.stepIdx = 0
			return m, m.runPipeline()
		case "s", "S":
			return m, m.saveOnly()
		case "d", "D":
			if m.recorder != nil {
				m.recorder.Stop()
			}
			return m, tea.Quit
		case "ctrl+c":
			if m.recorder != nil {
				m.recorder.Stop()
			}
			return m, tea.Quit
		}

	case phaseResults:
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) startRecording() tea.Cmd {
	return func() tea.Msg {
		// recorder is started in Run() before tea.NewProgram
		return tickMsg(time.Now())
	}
}

func (m model) transcribeChunk() tea.Cmd {
	snapshot := m.recorder.Snapshot(0)
	elapsed := m.elapsed
	sampleRate := m.opts.SampleRate
	c := m.opts.Client

	return func() tea.Msg {
		tmp, err := audio.WriteWAVTemp(snapshot, sampleRate, 2)
		if err != nil {
			return transcriptMsg{}
		}
		defer os.Remove(tmp)

		result, err := c.Transcribe(tmp)
		if err != nil {
			return transcriptMsg{}
		}
		return transcriptMsg{text: result.Text, ts: elapsed}
	}
}

func (m model) runPipeline() tea.Cmd {
	return func() tea.Msg {
		samples := m.recorder.Stop()
		if err := audio.WriteWAV(m.opts.OutputPath, samples, m.opts.SampleRate, 2); err != nil {
			return pipelineDoneMsg{err: err}
		}

		result, err := m.opts.Client.Annotate(m.opts.OutputPath, 0.5, m.opts.Template)
		if err != nil {
			return pipelineDoneMsg{err: err}
		}

		if m.opts.DB != nil {
			dur := time.Duration(len(samples)/2/m.opts.SampleRate) * time.Second
			meetingID, _ := m.opts.DB.InsertMeeting(&store.Meeting{
				Name:        m.opts.Name,
				RecordedAt:  time.Now(),
				DurationS:   dur.Seconds(),
				Template:    m.opts.Template,
				AudioPath:   m.opts.OutputPath,
				NumSpeakers: result.RawVAD.NumSpeakers,
			})
			for _, seg := range result.Segments {
				m.opts.DB.InsertSegment(&store.Segment{
					MeetingID:    meetingID,
					SpeakerLabel: seg.Speaker,
					StartS:       seg.Start,
					EndS:         seg.End,
					Text:         seg.Text,
				})
			}
			m.opts.DB.InsertSummary(&store.Summary{
				MeetingID: meetingID,
				Template:  m.opts.Template,
				Content:   result.Summary,
			})
		}

		return pipelineDoneMsg{result: result}
	}
}

func (m model) saveOnly() tea.Cmd {
	return func() tea.Msg {
		samples := m.recorder.Stop()
		audio.WriteWAV(m.opts.OutputPath, samples, m.opts.SampleRate, 2)
		if m.opts.DB != nil {
			dur := time.Duration(len(samples)/2/m.opts.SampleRate) * time.Second
			m.opts.DB.InsertMeeting(&store.Meeting{
				Name:       m.opts.Name,
				RecordedAt: time.Now(),
				DurationS:  dur.Seconds(),
				Template:   m.opts.Template,
				AudioPath:  m.opts.OutputPath,
			})
		}
		return tea.Quit()
	}
}

// --- Views ---

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	switch m.phase {
	case phaseLaunch:
		return m.viewLaunch()
	case phaseRecording:
		return m.viewRecording()
	case phaseConfirm:
		return m.viewConfirm()
	case phaseProcessing:
		return m.viewProcessing()
	case phaseResults:
		return m.viewResults()
	}
	return ""
}

func (m model) viewLaunch() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("scrib") + "  " + dimStyle.Render("select a meeting to record") + "\n\n")

	for i, item := range m.eventItems {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			prefix = "▸ "
			style = selStyle
		}
		label := item.label
		if item.event != nil && item.event.IsNow() {
			label = nowStyle.Render("NOW ") + label
		}
		sb.WriteString(prefix + style.Render(label) + "\n")
	}

	sb.WriteString("\n" + dimStyle.Render("  ↑/↓: navigate  enter: select  q: quit"))
	return sb.String()
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
	if m.recorder != nil {
		inputDev := audio.GetInputDeviceName()
		outputDev := audio.GetOutputDeviceName()
		deviceInfo = dimStyle.Render(fmt.Sprintf("  mic: %s  │  out: %s", inputDev, outputDev))
	}

	waveform := ""
	if m.recorder != nil {
		waveform = renderWaveform(m.recorder.MicLevel(), m.recorder.SysLevel(), max(m.width-6, 20))
	}

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-6, 1)).
		Render(m.viewport.View())

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", title, content, waveform, status, deviceInfo)
}

func (m model) viewConfirm() string {
	elapsed := fmt.Sprintf("%02d:%02d", int(m.elapsed.Minutes()), int(m.elapsed.Seconds())%60)
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))

	body := fmt.Sprintf(`
  Recording stopped (%s)

  What would you like to do?

  %s  Process (VAD + STT + summarise)
  %s  Save audio only
  %s  Discard

`, elapsed,
		selStyle.Render("[P]"),
		selStyle.Render("[S]"),
		selStyle.Render("[D]"))

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-4, 1)).
		Render(body)

	return fmt.Sprintf("%s\n%s", title, content)
}

func (m model) viewProcessing() string {
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))

	var sb strings.Builder
	sb.WriteString("\n  Processing...\n\n")
	for i, step := range m.steps {
		if step.done {
			sb.WriteString(fmt.Sprintf("  %s %s\n", checkStyle.Render("✓"), step.label))
		} else if i == m.stepIdx {
			sb.WriteString(fmt.Sprintf("  %s %s\n", m.spinner.View(), step.label))
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(step.label)))
		}
	}

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-4, 1)).
		Render(sb.String())

	return fmt.Sprintf("%s\n%s", title, content)
}

func (m model) viewResults() string {
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))
	status := dimStyle.Render("  ↑/↓: scroll  │  q: exit")

	content := borderStyle.
		Width(max(m.width-2, 10)).
		Height(max(m.height-4, 1)).
		Render(m.resultVP.View())

	return fmt.Sprintf("%s\n%s\n%s", title, content, status)
}

func (m model) renderTranscript() string {
	if len(m.transcript) == 0 {
		return dimStyle.Render("\n  Listening... transcript will appear here\n")
	}
	var sb strings.Builder
	for _, line := range m.transcript {
		mins := int(line.ts.Minutes())
		secs := int(line.ts.Seconds()) % 60
		sb.WriteString(fmt.Sprintf("  [%02d:%02d] %s\n", mins, secs, line.text))
	}
	return sb.String()
}

func (m model) renderResults() string {
	if m.result == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Duration: %s | Speakers: %d\n", m.result.Duration.Round(time.Second), m.result.RawVAD.NumSpeakers))
	sb.WriteString(fmt.Sprintf("  Audio: %s\n\n", m.opts.OutputPath))
	sb.WriteString(m.result.Summary)
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

func sanitizeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '-'
		}
		return r
	}, s)
	if len(s) > 60 {
		s = s[:60]
	}
	return time.Now().Format("2006-01-02") + "-" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "-"))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Run starts the scrib TUI.
func Run(opts Options) error {
	rec, err := audio.NewRecorder(opts.SampleRate)
	if err != nil {
		return fmt.Errorf("init recorder: %w", err)
	}

	if err := rec.Start(); err != nil {
		return fmt.Errorf("start recording: %w", err)
	}

	m := initialModel(opts)
	m.recorder = rec
	m.startTime = time.Now()
	m.lastChunk = time.Now()

	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	fm := finalModel.(model)
	if fm.err != nil {
		return fm.err
	}

	if fm.phase == phaseResults {
		fmt.Printf("Saved: %s\n", opts.OutputPath)
	}

	return nil
}
