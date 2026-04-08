// Package tui provides the BubbleTea-based TUI for scrib.
// Three phases: recording (live transcript), processing (pipeline), results (rendered markdown).
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/solanyn/mono/scrib/audio"
	"github.com/solanyn/mono/scrib/client"
	"github.com/solanyn/mono/scrib/store"
)

type phase int

const (
	phaseRecording phase = iota
	phaseProcessing
	phaseResults
)

// Options configures the TUI.
type Options struct {
	Name       string
	OutputPath string
	SampleRate int
	AudioURL   string
	GatewayURL string
	Client     *client.Client
	Template   string
	DB         *store.DB
}

type model struct {
	opts      Options
	recorder  *audio.Recorder
	phase     phase
	width     int
	height    int
	startTime time.Time
	elapsed   time.Duration

	// recording phase
	transcript []transcriptLine
	viewport   viewport.Model

	// processing phase
	spinner  spinner.Model
	steps    []processStep
	stepIdx  int

	// results phase
	result    *client.AnnotateResult
	resultVP  viewport.Model
	meetingID int64

	err error
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
type pipelineDoneMsg struct {
	result *client.AnnotateResult
	err    error
}
type stepDoneMsg int

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func initialModel(opts Options) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(80, 20)

	return model{
		opts:      opts,
		phase:     phaseRecording,
		startTime: time.Now(),
		viewport:  vp,
		spinner:   sp,
		steps: []processStep{
			{label: "Saving audio"},
			{label: "Speaker diarisation"},
			{label: "Transcription"},
			{label: "Summarising"},
		},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.phase == phaseRecording {
				m.phase = phaseProcessing
				m.stepIdx = 0
				return m, m.runPipeline()
			}
			if m.phase == phaseResults {
				return m, tea.Quit
			}
		case "q":
			if m.phase == phaseResults {
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := m.height - 4
		m.viewport.Width = m.width - 4
		m.viewport.Height = contentH
		m.resultVP.Width = m.width - 4
		m.resultVP.Height = contentH

	case tickMsg:
		if m.phase == phaseRecording {
			m.elapsed = time.Since(m.startTime)
		}
		cmds = append(cmds, tickCmd())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case stepDoneMsg:
		idx := int(msg)
		if idx < len(m.steps) {
			m.steps[idx].done = true
			m.stepIdx = idx + 1
		}

	case pipelineDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.result = msg.result
		m.phase = phaseResults
		m.resultVP = viewport.New(m.width-4, m.height-4)
		m.resultVP.SetContent(m.renderResults())
	}

	// Update viewport for current phase
	if m.phase == phaseRecording {
		m.viewport.SetContent(m.renderTranscript())
		m.viewport.GotoBottom()
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

func (m model) runPipeline() tea.Cmd {
	return func() tea.Msg {
		// Step 0: save audio
		samples := m.recorder.Stop()
		if err := audio.WriteWAV(m.opts.OutputPath, samples, m.opts.SampleRate, 2); err != nil {
			return pipelineDoneMsg{err: err}
		}

		// Steps 1-3: annotate (VAD + STT + summarise)
		result, err := m.opts.Client.Annotate(m.opts.OutputPath, 0.5, m.opts.Template)
		if err != nil {
			return pipelineDoneMsg{err: err}
		}

		// Store in database
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

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	recStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	checkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
)

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	switch m.phase {
	case phaseRecording:
		return m.viewRecording()
	case phaseProcessing:
		return m.viewProcessing()
	case phaseResults:
		return m.viewResults()
	}
	return ""
}

func (m model) viewRecording() string {
	elapsed := fmt.Sprintf("%02d:%02d", int(m.elapsed.Minutes()), int(m.elapsed.Seconds())%60)
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))
	rec := recStyle.Render("● REC")
	status := dimStyle.Render(fmt.Sprintf("  %s  %s  │  ctrl+c: stop & process", rec, elapsed))

	content := borderStyle.
		Width(m.width - 2).
		Height(m.height - 4).
		Render(m.viewport.View())

	return fmt.Sprintf("%s\n%s\n%s", title, content, status)
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
		Width(m.width - 2).
		Height(m.height - 4).
		Render(sb.String())

	return fmt.Sprintf("%s\n%s", title, content)
}

func (m model) viewResults() string {
	title := titleStyle.Render(fmt.Sprintf("─ %s ", m.opts.Name))
	status := dimStyle.Render("  ↑/↓: scroll  │  q: exit")

	content := borderStyle.
		Width(m.width - 2).
		Height(m.height - 4).
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
	dur := m.result.Duration
	sb.WriteString(fmt.Sprintf("  Duration: %s | Speakers: %d\n", dur.Round(time.Second), m.result.RawVAD.NumSpeakers))
	sb.WriteString(fmt.Sprintf("  Audio: %s\n\n", m.opts.OutputPath))
	sb.WriteString(m.result.Summary)
	sb.WriteString("\n")

	return sb.String()
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

	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	fm := finalModel.(model)
	if fm.err != nil {
		return fm.err
	}

	fmt.Printf("Saved: %s\n", opts.OutputPath)
	mdPath := strings.TrimSuffix(opts.OutputPath, ".wav") + ".md"
	fmt.Printf("Notes: %s\n", mdPath)

	return nil
}
