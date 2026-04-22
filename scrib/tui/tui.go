package tui

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/solanyn/mono/scrib/audio"
)

type phase int

const (
	phaseRecording phase = iota
	phaseProcessing
	phaseResults
)

type Options struct {
	Name       string
	OutputPath string
	SampleRate int
	ServerURL  string
	Template   string
}

type UploadFunc func(samples []int16, opts Options) (string, error)

type model struct {
	opts      Options
	recorder  *audio.Recorder
	phase     phase
	width     int
	height    int
	startTime time.Time
	elapsed   time.Duration
	uploadFn  UploadFunc

	spinner spinner.Model
	steps   []processStep
	stepIdx int

	result *pipelineResult
	resultVP viewport.Model

	err error
}

type pipelineResult struct {
	UUID     string
	Duration time.Duration
}

type processStep struct {
	label string
	done  bool
}

type tickMsg time.Time
type pipelineDoneMsg struct {
	result *pipelineResult
	err    error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	meterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	waveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func initialModel(opts Options, uploadFn UploadFunc) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		opts:  opts,
		phase: phaseRecording,
		spinner: sp,
		uploadFn: uploadFn,
		steps: []processStep{
			{label: "Saving audio"},
			{label: "Uploading to server"},
			{label: "Requesting processing"},
		},
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

	case pipelineDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
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

func (m model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.phase {
	case phaseRecording:
		if key == "ctrl+c" {
			m.phase = phaseProcessing
			m.stepIdx = 0
			return m, m.runPipeline()
		}

	case phaseResults:
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) runPipeline() tea.Cmd {
	return func() tea.Msg {
		samples := m.recorder.Stop()
		if err := audio.WriteWAV(m.opts.OutputPath, samples, m.opts.SampleRate, 2); err != nil {
			return pipelineDoneMsg{err: err}
		}

		dur := time.Duration(len(samples)/2/m.opts.SampleRate) * time.Second

		var uuid string
		if m.uploadFn != nil && m.opts.ServerURL != "" {
			monoSamples := audio.StereoToMono(samples)
			var err error
			uuid, err = m.uploadFn(monoSamples, m.opts)
			if err != nil {
				return pipelineDoneMsg{err: fmt.Errorf("upload: %w", err)}
			}
		}

		return pipelineDoneMsg{result: &pipelineResult{UUID: uuid, Duration: dur}}
	}
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	switch {
	case m.err != nil:
		v.SetContent(fmt.Sprintf("Error: %v\n", m.err))
	case m.phase == phaseRecording:
		v.SetContent(m.viewRecording())
	case m.phase == phaseProcessing:
		v.SetContent(m.viewProcessing())
	case m.phase == phaseResults:
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
		Render(dimStyle.Render("\n  Recording... press ctrl+c to stop\n"))

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", title, content, waveform, status, deviceInfo)
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
	if fm.err != nil {
		return fm.err
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
