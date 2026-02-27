package channels

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"

	"github.com/develder/millibee/pkg/bus"
	"github.com/develder/millibee/pkg/config"
	"github.com/develder/millibee/pkg/logger"
	"github.com/develder/millibee/pkg/tui"
)

// SSHChannel implements the Channel interface using a Wish SSH server
// that serves a Bubble Tea TUI to each connecting client.
type SSHChannel struct {
	*BaseChannel
	sshConfig   config.SSHConfig
	server      *ssh.Server
	listener    net.Listener
	sessions    map[string]chan string // chatID -> outbound channel
	streamChans map[string]chan string // chatID -> streaming chunk channel
	sessionsMu  sync.RWMutex
}

// NewSSHChannel creates a new SSH channel.
func NewSSHChannel(cfg config.SSHConfig, msgBus *bus.MessageBus) (*SSHChannel, error) {
	base := NewBaseChannel("ssh", cfg, msgBus, cfg.AllowFrom)

	ch := &SSHChannel{
		BaseChannel: base,
		sshConfig:   cfg,
		sessions:    make(map[string]chan string),
		streamChans: make(map[string]chan string),
	}

	return ch, nil
}

// Start starts the Wish SSH server.
func (c *SSHChannel) Start(ctx context.Context) error {
	address := c.sshConfig.Address
	if address == "" {
		address = "0.0.0.0:2222"
	}

	opts := []ssh.Option{
		wish.WithAddress(address),
		wish.WithMiddleware(
			bubbletea.Middleware(c.teaHandler),
			activeterm.Middleware(),
		),
	}

	hostKeyPath := c.sshConfig.HostKeyPath
	if hostKeyPath == "" {
		// Default to a writable location inside the config directory
		homeDir, _ := os.UserHomeDir()
		hostKeyPath = filepath.Join(homeDir, ".millibee", "ssh_host_key")
	}
	opts = append(opts, wish.WithHostKeyPath(hostKeyPath))

	if c.hasPasswordAuth() {
		opts = append(opts, wish.WithPasswordAuth(func(_ ssh.Context, password string) bool {
			return c.validatePassword("", password)
		}))
	}

	srv, err := wish.NewServer(opts...)
	if err != nil {
		return fmt.Errorf("create SSH server: %w", err)
	}
	c.server = srv

	// Listen manually so we can retrieve the actual address (important for port 0)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	c.listener = ln

	c.setRunning(true)
	logger.InfoCF("ssh", "SSH channel started", map[string]any{
		"address": ln.Addr().String(),
	})

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			logger.ErrorCF("ssh", "SSH server error", map[string]any{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// Stop shuts down the SSH server.
func (c *SSHChannel) Stop(ctx context.Context) error {
	logger.InfoC("ssh", "Stopping SSH channel")
	c.setRunning(false)

	if c.server != nil {
		if err := c.server.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			return fmt.Errorf("shutdown SSH server: %w", err)
		}
	}

	return nil
}

// Send delivers an outbound message to the appropriate SSH session.
func (c *SSHChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	c.sessionsMu.RLock()
	outChan, ok := c.sessions[msg.ChatID]
	c.sessionsMu.RUnlock()

	if !ok {
		return fmt.Errorf("no SSH session for chatID %q", msg.ChatID)
	}

	select {
	case outChan <- msg.Content:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending to SSH session %q", msg.ChatID)
	}
}

// SendChunk delivers a streaming text chunk to the SSH session.
func (c *SSHChannel) SendChunk(ctx context.Context, chatID string, chunk string) error {
	c.sessionsMu.RLock()
	ch, ok := c.streamChans[chatID]
	c.sessionsMu.RUnlock()

	if !ok {
		return nil
	}

	select {
	case ch <- chunk:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil // non-blocking, drop chunk if buffer full
	}
}

// FlushStream signals end-of-stream by closing the chunk channel.
func (c *SSHChannel) FlushStream(ctx context.Context, chatID string) error {
	c.sessionsMu.Lock()
	if ch, ok := c.streamChans[chatID]; ok {
		close(ch)
		delete(c.streamChans, chatID)
	}
	c.sessionsMu.Unlock()
	return nil
}

// registerStreamChan creates a streaming chunk channel for a chat session.
func (c *SSHChannel) registerStreamChan(chatID string) chan string {
	ch := make(chan string, 256)
	c.sessionsMu.Lock()
	c.streamChans[chatID] = ch
	c.sessionsMu.Unlock()
	return ch
}

// ListenAddr returns the actual address the server is listening on.
// Useful when started with port 0 (random port).
func (c *SSHChannel) ListenAddr() string {
	if c.listener != nil {
		return c.listener.Addr().String()
	}
	return ""
}

// registerSession registers an outbound channel for a chat session.
func (c *SSHChannel) registerSession(chatID string, outChan chan string) {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()
	c.sessions[chatID] = outChan
}

// unregisterSession removes a chat session.
func (c *SSHChannel) unregisterSession(chatID string) {
	c.sessionsMu.Lock()
	defer c.sessionsMu.Unlock()
	delete(c.sessions, chatID)
}

// hasPasswordAuth returns true if password authentication is configured.
func (c *SSHChannel) hasPasswordAuth() bool {
	return c.sshConfig.Password != ""
}

// validatePassword checks if the provided password matches the configured password.
func (c *SSHChannel) validatePassword(_ string, password string) bool {
	return password == c.sshConfig.Password
}

// teaHandler creates a Bubble Tea model for each SSH session.
func (c *SSHChannel) teaHandler(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
	username := sess.User()
	chatID := fmt.Sprintf("ssh:%s", username)

	renderer := bubbletea.MakeRenderer(sess)
	styles := tui.NewStyles(renderer)

	outChan := make(chan string, 64)
	c.registerSession(chatID, outChan)

	model := newSSHModel(c, username, chatID, styles, outChan, renderer)

	return model, []tea.ProgramOption{tea.WithAltScreen()}
}

// sshOutboundMsg carries a response delivered via the outbound channel.
type sshOutboundMsg struct{ content string }

// sshStreamChunkMsg carries an incremental text chunk during streaming.
type sshStreamChunkMsg struct{ chunk string }

// sshStreamDoneMsg signals that streaming is complete.
type sshStreamDoneMsg struct{}

// sshSessionClosedMsg signals that the outbound channel was closed.
type sshSessionClosedMsg struct{}

// listenOutbound returns a tea.Cmd that reads from the outbound channel.
func listenOutbound(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		content, ok := <-ch
		if !ok {
			return sshSessionClosedMsg{}
		}
		return sshOutboundMsg{content: content}
	}
}

// listenStreamChunks returns a tea.Cmd that reads from the stream channel.
func listenStreamChunks(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return sshStreamDoneMsg{}
		}
		return sshStreamChunkMsg{chunk: chunk}
	}
}

// sshModel is the Bubble Tea model for an SSH session.
// It provides a full chat TUI and connects to the MessageBus
// via the SSHChannel's HandleMessage/Send flow.
type sshModel struct {
	channel  *SSHChannel
	username string
	chatID   string
	styles   tui.Styles
	outChan  chan string
	renderer *lipgloss.Renderer

	messages        []tui.ChatMessage
	textarea        textarea.Model
	viewport        viewport.Model
	spinner         spinner.Model
	processing       bool
	streaming        bool              // true while receiving chunks
	streamChan       <-chan string      // chunk channel for current stream
	streamChunkCount int               // chunks received; used for render throttling
	width           int
	height          int
	ready           bool
	glamourRenderer *glamour.TermRenderer
}

func newSSHModel(ch *SSHChannel, username, chatID string, styles tui.Styles, outChan chan string, renderer *lipgloss.Renderer) *sshModel {
	ta := textarea.New()
	ta.Placeholder = "Ask something or give a command... (Enter to send, Ctrl+C to quit)"
	ta.Focus()
	ta.CharLimit = 4096
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	gr, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(80))
	if err != nil || gr == nil {
		// AutoStyle failed (common over SSH) — fall back to dark theme
		gr, _ = glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(80))
	}

	return &sshModel{
		channel:         ch,
		username:        username,
		chatID:          chatID,
		styles:          styles,
		outChan:         outChan,
		renderer:        renderer,
		messages:        []tui.ChatMessage{},
		textarea:        ta,
		spinner:         sp,
		glamourRenderer: gr,
	}
}

func (m *sshModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, listenOutbound(m.outChan))
}

func (m *sshModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyPgUp:
			m.viewport.HalfViewUp()
			return m, tea.Batch(cmds...)
		case tea.KeyPgDown:
			m.viewport.HalfViewDown()
			return m, tea.Batch(cmds...)
		case tea.KeyUp:
			// Scroll viewport when waiting for response or textarea is empty
			if m.processing || m.textarea.Value() == "" {
				m.viewport.LineUp(3)
				return m, tea.Batch(cmds...)
			}
		case tea.KeyDown:
			if m.processing || m.textarea.Value() == "" {
				m.viewport.LineDown(3)
				return m, tea.Batch(cmds...)
			}
		case tea.KeyCtrlC:
			m.channel.unregisterSession(m.chatID)
			return m, tea.Quit
		case tea.KeyTab:
			if !m.processing {
				m.completeCommand()
				return m, nil
			}
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

	case sshStreamChunkMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			m.messages[len(m.messages)-1].Content += msg.chunk
		} else {
			m.messages = append(m.messages, tui.ChatMessage{
				Role:    "assistant",
				Content: msg.chunk,
			})
		}
		m.streamChunkCount++
		// Throttle: only re-render every 10 chunks to reduce SSH escape-sequence traffic.
		// Streaming still happens per-token; the user sees updates every ~10 tokens.
		if m.ready && m.streamChunkCount%10 == 1 {
			m.viewport.SetContent(m.renderMessagesRaw())
			m.viewport.GotoBottom()
		}
		return m, listenStreamChunks(m.streamChan)

	case sshStreamDoneMsg:
		m.streaming = false
		// Continue listening for the final outbound message
		return m, listenOutbound(m.outChan)

	case sshOutboundMsg:
		m.processing = false
		if m.streaming {
			// Should not happen, but handle gracefully
			m.streaming = false
		}
		// If we were streaming, replace the streamed content with the
		// final glamour-rendered version
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			m.messages[len(m.messages)-1].Content = msg.content
		} else {
			m.messages = append(m.messages, tui.ChatMessage{
				Role:    "assistant",
				Content: msg.content,
			})
		}
		if m.ready {
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}
		m.textarea.Focus()
		return m, listenOutbound(m.outChan)

	case sshSessionClosedMsg:
		m.channel.unregisterSession(m.chatID)
		return m, tea.Quit

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

func (m *sshModel) sendMessage() (*sshModel, tea.Cmd) {
	input := m.textarea.Value()
	if input == "" {
		return m, nil
	}

	m.messages = append(m.messages, tui.ChatMessage{
		Role:    "user",
		Content: input,
	})
	m.textarea.Reset()
	m.processing = true
	m.streaming = true
	m.streamChunkCount = 0

	if m.ready {
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	// Register a stream channel for chunks
	streamCh := m.channel.registerStreamChan(m.chatID)
	m.streamChan = streamCh

	// Publish to MessageBus via BaseChannel
	m.channel.HandleMessage(m.username, m.chatID, input, nil, nil)

	return m, tea.Batch(m.spinner.Tick, listenStreamChunks(streamCh))
}

func (m *sshModel) renderMessages() string {
	if len(m.messages) == 0 {
		return ""
	}

	var s string
	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			s += m.styles.User.Render("You: "+msg.Content) + "\n\n"
		case "assistant":
			rendered := msg.Content
			if m.glamourRenderer != nil {
				if r, err := m.glamourRenderer.Render(msg.Content); err == nil {
					rendered = r
				}
			}
			s += m.styles.Assistant.Render("Assistant:") + "\n" + rendered + "\n"
		case "error":
			s += m.styles.Error.Render(msg.Content) + "\n\n"
		}
	}
	return s
}

// renderMessagesRaw renders messages without glamour markdown rendering.
// Used during streaming to avoid re-rendering partial markdown on every token.
func (m *sshModel) renderMessagesRaw() string {
	if len(m.messages) == 0 {
		return ""
	}
	var s string
	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			s += m.styles.User.Render("You: "+msg.Content) + "\n\n"
		case "assistant":
			s += m.styles.Assistant.Render("Assistant:") + "\n" + msg.Content + "\n"
		case "error":
			s += m.styles.Error.Render(msg.Content) + "\n\n"
		}
	}
	return s
}

func (m *sshModel) View() string {
	if !m.ready {
		return fmt.Sprintf("Connecting as %s...", m.username)
	}

	var status string
	if m.processing {
		status = m.styles.Status.Render(m.spinner.View() + " Thinking...")
	} else if hint := m.commandHint(); hint != "" {
		status = m.styles.Status.Render(hint)
	} else {
		status = m.styles.Status.Render("Ready | Ctrl+C to quit")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		status,
		m.textarea.View(),
	)
}

// commandHint returns a status bar hint when the textarea starts with /.
func (m *sshModel) commandHint() string {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" || input[0] != '/' {
		return ""
	}
	matches := tui.MatchCommands(input)
	if len(matches) == 0 {
		return ""
	}
	names := make([]string, len(matches))
	for i, c := range matches {
		names[i] = c.Name
	}
	return "Commands: " + strings.Join(names, "  ")
}

// completeCommand auto-completes the command in textarea when there is exactly one match.
func (m *sshModel) completeCommand() {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" || input[0] != '/' {
		return
	}
	matches := tui.MatchCommands(input)
	if len(matches) == 1 {
		m.textarea.SetValue(matches[0].Name)
	}
}
