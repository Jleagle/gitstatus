package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/viper"
)

const (
	recentDone  = 6
	runningMax  = 10
	progressW   = 40
	livePathMax = 48
)

type repoStageMsg struct{ path, stage string }
type repoDoneMsg struct{ row rowItem }
type allDoneMsg struct{}

// teaReporter forwards worker progress into the running program
type teaReporter struct{ p *tea.Program }

func (t teaReporter) stage(path, stage string) { t.p.Send(repoStageMsg{path, stage}) }
func (t teaReporter) done(row rowItem)         { t.p.Send(repoDoneMsg{row}) }

type liveModel struct {
	baseDir     string
	total       int
	pathW       int
	start       time.Time
	spinner     spinner.Model
	running     []string // paths in the order they started
	stages      map[string]string
	done        []rowItem
	finished    bool
	interrupted bool
}

func newLiveModel(repos []repoItem, baseDir string) *liveModel {

	pathW := 0
	for _, r := range repos {
		pathW = max(pathW, len([]rune(rowItem{path: r.path}.displayPath(baseDir))))
	}

	return &liveModel{
		baseDir: baseDir,
		total:   len(repos),
		pathW:   min(pathW, livePathMax),
		start:   time.Now(),
		spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(blue)),
		stages:  map[string]string{},
	}
}

func (m *liveModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *liveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.interrupted = true
			return m, tea.Quit
		}
	case repoStageMsg:
		if _, ok := m.stages[msg.path]; !ok {
			m.running = append(m.running, msg.path)
		}
		m.stages[msg.path] = msg.stage
	case repoDoneMsg:
		delete(m.stages, msg.row.path)
		for i, p := range m.running {
			if p == msg.row.path {
				m.running = append(m.running[:i], m.running[i+1:]...)
				break
			}
		}
		m.done = append(m.done, msg.row)
	case allDoneMsg:
		m.finished = true
		results := strings.TrimSuffix(renderResults(m.done, m.baseDir, time.Since(m.start)), "\n")
		return m, tea.Sequence(tea.Println(results), tea.Quit)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *liveModel) View() tea.View {

	// Results are printed above the live region, which is then cleared
	if m.finished {
		return tea.NewView("")
	}

	pull := dim.Render("off")
	if viper.GetBool(fPull) {
		pull = green.Render("on")
	}
	header := badge.Render("gitstatus") + " " + dim.Render("scanning ") + bright.Render(tildeHome(m.baseDir)) +
		dim.Render(fmt.Sprintf(" · %d repos · %d workers · pull ", m.total, concurrency())) + pull + "\n\n"

	var b strings.Builder

	for _, p := range m.running[:min(len(m.running), runningMax)] {
		b.WriteString(m.spinner.View() + " " + pad(bright.Render(m.livePath(p)), m.pathW+2) + blue.Render(m.stages[p]) + "\n")
	}
	if more := len(m.running) - runningMax; more > 0 {
		b.WriteString(dim.Render(fmt.Sprintf("  … %d more running", more)) + "\n")
	}
	if len(m.running) > 0 {
		b.WriteString("\n")
	}

	from := max(0, len(m.done)-recentDone)
	if from > 0 {
		b.WriteString(dim.Render(fmt.Sprintf("  … %d more done", from)) + "\n")
	}
	for _, r := range m.done[from:] {
		b.WriteString(m.doneLine(r) + "\n")
	}
	if len(m.done) > 0 {
		b.WriteString("\n")
	}

	b.WriteString(m.progressLine() + "\n")
	b.WriteString(m.countsLine())

	// A constant frame height avoids bubbletea's inline renderer leaving stale
	// lines behind when the frame resizes
	frame := header + b.String()
	if gap := m.frameHeight() - strings.Count(frame, "\n") - 1; gap > 0 {
		frame = header + strings.Repeat("\n", gap) + b.String()
	}
	return tea.NewView(frame)
}

// frameHeight is the tallest the live view can get: header, running, done and progress sections
func (m *liveModel) frameHeight() int {
	running := min(concurrency(), m.total)
	if running > runningMax {
		running = runningMax + 1
	}
	done := min(recentDone+1, m.total)
	return 2 + running + 1 + done + 1 + 2
}

func (m *liveModel) livePath(path string) string {
	return truncate(rowItem{path: path}.displayPath(m.baseDir), m.pathW)
}

func (m *liveModel) doneLine(r rowItem) string {

	path := pad(m.livePath(r.path), m.pathW+2)

	if !r.show() {
		return dim.Render("✓ " + path + "up to date")
	}

	info := categories[r.category()]
	var detail string
	switch {
	case r.error != nil:
		detail = red.Render(r.errorText())
	case r.skipped != "":
		detail = orange.Render("skipped: " + r.skipped)
	case r.updated:
		detail = green.Render("updated")
	case !r.isMain():
		detail = purple.Render(r.displayBranch())
	}
	if changes := formatChanges(r, true); changes != "" {
		detail = strings.TrimSpace(detail + " " + changes)
	}

	return info.style.Render(info.glyph) + " " + bright.Render(path) + detail
}

func (m *liveModel) progressLine() string {

	n := len(m.done)
	filled := 0
	if m.total > 0 {
		filled = progressW * n / m.total
	}
	bar := blue.Render(strings.Repeat("━", filled)) + track.Render(strings.Repeat("━", progressW-filled))

	return bar + "  " + bright.Render(fmt.Sprint(n)) + dim.Render(fmt.Sprintf("/%d  %.1fs", m.total, time.Since(m.start).Seconds()))
}

func (m *liveModel) countsLine() string {

	counts := map[category]int{}
	for _, r := range m.done {
		if r.show() {
			counts[r.category()]++
		}
	}

	var parts []string
	for c := catError; c < catClean; c++ {
		if n := counts[c]; n > 0 {
			parts = append(parts, categories[c].style.Render(fmt.Sprintf("%d %s", n, categories[c].summary)))
		}
	}
	if queued := m.total - len(m.done) - len(m.running); queued > 0 {
		parts = append(parts, dim.Render(fmt.Sprintf("%d queued", queued)))
	}
	return strings.Join(parts, dim.Render(" · "))
}

// runLive processes the repos while rendering live progress, ending on the
// results; ctrl+c exits immediately
func runLive(repos []repoItem, baseDir string) {

	model := newLiveModel(repos, baseDir)
	p := tea.NewProgram(model, programOptions()...)

	go func() {
		pullRepos(repos, teaReporter{p})
		p.Send(allDoneMsg{})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if model.interrupted {
		os.Exit(130)
	}
}

// programOptions works around JetBrains' terminal ignoring CSI Z (cursor
// backward tab), which leaves stale text behind; the renderer doesn't use it
// for TERM=linux, so colors are detected from the real TERM instead
func programOptions() []tea.ProgramOption {
	if os.Getenv("TERMINAL_EMULATOR") != "JetBrains-JediTerm" {
		return nil
	}
	return []tea.ProgramOption{
		tea.WithColorProfile(colorprofile.Detect(os.Stdout, os.Environ())),
		tea.WithEnvironment(append(os.Environ(), "TERM=linux")),
	}
}

func tildeHome(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
