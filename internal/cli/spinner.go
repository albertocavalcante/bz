package cli

import (
	"io"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// spinnerModel is the bubbletea model for the spinner.
type spinnerModel struct {
	spinner  spinner.Model
	message  string
	done     bool
	err      error
	fn       func() error
	quitting bool
}

// Init implements tea.Model.
func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runTask)
}

// runTask runs the provided function in the background.
func (m spinnerModel) runTask() tea.Msg {
	err := m.fn()
	return taskDoneMsg{err: err}
}

// taskDoneMsg signals that the task has completed.
type taskDoneMsg struct {
	err error
}

// Update implements tea.Model.
func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case taskDoneMsg:
		m.done = true
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Allow Ctrl+C to quit
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}
	return m.spinner.View() + " " + m.message
}

// WithSpinner runs a function while displaying a spinner with the given message.
// The spinner is displayed on stderr to not interfere with command output.
func WithSpinner(message string, fn func() error) error {
	return WithSpinnerWriter(os.Stderr, message, fn)
}

// WithSpinnerWriter runs a function while displaying a spinner with the given message.
// The spinner is written to the provided writer.
func WithSpinnerWriter(w io.Writer, message string, fn func() error) error {
	// In quiet mode, just run the function without spinner
	if IsQuiet() {
		return fn()
	}

	// Check if we're in a TTY - if not, just run without spinner
	if f, ok := w.(*os.File); ok {
		if !isTerminal(f) {
			return fn()
		}
	} else {
		// Not a file, just run without spinner
		return fn()
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // Blue

	m := spinnerModel{
		spinner: s,
		message: message,
		fn:      fn,
	}

	p := tea.NewProgram(m, tea.WithOutput(w))
	finalModel, err := p.Run()
	if err != nil {
		// If the TUI fails, just run the function directly
		return fn()
	}

	final := finalModel.(spinnerModel)
	return final.err
}

// isTerminal checks if the file is a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// SimpleSpinner provides a simple non-TUI spinner for testing and simple cases.
type SimpleSpinner struct {
	message string
	writer  io.Writer
	done    chan struct{}
	running bool
}

// NewSimpleSpinner creates a new simple spinner.
func NewSimpleSpinner(w io.Writer, message string) *SimpleSpinner {
	return &SimpleSpinner{
		message: message,
		writer:  w,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *SimpleSpinner) Start() {
	if IsQuiet() {
		return
	}

	s.running = true
	frames := []string{".", "..", "..."}
	go func() {
		i := 0
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				if f, ok := s.writer.(*os.File); ok && isTerminal(f) {
					// Clear line and write spinner
					_, _ = io.WriteString(s.writer, "\r"+s.message+frames[i%len(frames)]+"   \r")
				}
				i++
			}
		}
	}()
}

// Stop stops the spinner and clears the line.
func (s *SimpleSpinner) Stop() {
	if !s.running {
		return
	}
	close(s.done)
	s.running = false
	// Clear the spinner line
	if f, ok := s.writer.(*os.File); ok && isTerminal(f) {
		_, _ = io.WriteString(s.writer, "\r                                                  \r")
	}
}
