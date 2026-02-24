package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// AgentProcessor is the interface for processing messages. Matches agent.AgentLoop.ProcessDirect.
type AgentProcessor interface {
	ProcessDirect(ctx context.Context, input string, sessionKey string) (string, error)
}

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string // "user", "assistant", "error"
	Content string
}

// responseMsg carries a completed LLM response back to the Update loop.
type responseMsg struct {
	content string
	err     error
}

// Model is the Bubble Tea model for the PicoClaw TUI.
type Model struct {
	agent      AgentProcessor
	sessionKey string

	messages   []ChatMessage
	textarea   textarea.Model
	viewport   viewport.Model
	spinner    spinner.Model
	processing bool

	width  int
	height int
	ready  bool

	glamourRenderer *glamour.TermRenderer
}

// NewModel creates a new TUI model.
func NewModel(agent AgentProcessor, sessionKey string) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Ctrl+C to quit)"
	ta.Focus()
	ta.CharLimit = 4096
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = SpinnerStyle

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return Model{
		agent:           agent,
		sessionKey:      sessionKey,
		messages:        []ChatMessage{},
		textarea:        ta,
		spinner:         sp,
		glamourRenderer: renderer,
	}
}

// Init initializes the model and returns the initial command.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

// Update handles incoming messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.processing {
				return m.sendMessage()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 6 // textarea (3) + status (1) + padding (2)
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.viewport.SetContent(m.renderMessages())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}
		m.textarea.SetWidth(msg.Width - 4)
		return m, nil

	case responseMsg:
		m.processing = false
		if msg.err != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    "error",
				Content: fmt.Sprintf("Error: %v", msg.err),
			})
		} else {
			m.messages = append(m.messages, ChatMessage{
				Role:    "assistant",
				Content: msg.content,
			})
		}
		if m.ready {
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}
		m.textarea.Focus()
		return m, nil

	case spinner.TickMsg:
		if m.processing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update textarea when not processing
	if !m.processing {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	input := m.textarea.Value()
	if input == "" {
		return m, nil
	}

	m.messages = append(m.messages, ChatMessage{
		Role:    "user",
		Content: input,
	})
	m.textarea.Reset()
	m.processing = true

	if m.ready {
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	agent := m.agent
	sessionKey := m.sessionKey

	return m, tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			if agent == nil {
				return responseMsg{err: fmt.Errorf("no agent configured")}
			}
			ctx := context.Background()
			response, err := agent.ProcessDirect(ctx, input, sessionKey)
			return responseMsg{content: response, err: err}
		},
	)
}

func (m Model) renderMessages() string {
	if len(m.messages) == 0 {
		return ""
	}

	var s string
	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			s += UserStyle.Render("You: "+msg.Content) + "\n\n"
		case "assistant":
			rendered := msg.Content
			if m.glamourRenderer != nil {
				if r, err := m.glamourRenderer.Render(msg.Content); err == nil {
					rendered = r
				}
			}
			s += AssistantStyle.Render("Assistant:") + "\n" + rendered + "\n"
		case "error":
			s += ErrorStyle.Render(msg.Content) + "\n\n"
		}
	}
	return s
}

// View renders the current state of the model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var status string
	if m.processing {
		status = StatusStyle.Render(m.spinner.View() + " Thinking...")
	} else {
		status = StatusStyle.Render("Ready | Ctrl+C to quit")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		status,
		m.textarea.View(),
	)
}
