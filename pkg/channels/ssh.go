package channels

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/tui"
)

// SSHChannel implements the Channel interface using a Wish SSH server
// that serves a Bubble Tea TUI to each connecting client.
type SSHChannel struct {
	*BaseChannel
	sshConfig  config.SSHConfig
	server     *ssh.Server
	listener   net.Listener
	sessions   map[string]chan string // chatID -> outbound channel
	sessionsMu sync.RWMutex
}

// NewSSHChannel creates a new SSH channel.
func NewSSHChannel(cfg config.SSHConfig, msgBus *bus.MessageBus) (*SSHChannel, error) {
	base := NewBaseChannel("ssh", cfg, msgBus, cfg.AllowFrom)

	ch := &SSHChannel{
		BaseChannel: base,
		sshConfig:   cfg,
		sessions:    make(map[string]chan string),
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

	if c.sshConfig.HostKeyPath != "" {
		opts = append(opts, wish.WithHostKeyPath(c.sshConfig.HostKeyPath))
	}

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

// sshModel is the Bubble Tea model for an SSH session.
// It wraps the TUI chat components and connects to the MessageBus
// via the SSHChannel's HandleMessage/Send flow.
type sshModel struct {
	channel  *SSHChannel
	username string
	chatID   string
	styles   tui.Styles
	outChan  chan string
	renderer *lipgloss.Renderer

	// Embedded TUI state
	tuiModel tui.Model
}

func newSSHModel(ch *SSHChannel, username, chatID string, styles tui.Styles, outChan chan string, renderer *lipgloss.Renderer) *sshModel {
	return &sshModel{
		channel:  ch,
		username: username,
		chatID:   chatID,
		styles:   styles,
		outChan:  outChan,
		renderer: renderer,
	}
}

func (m *sshModel) Init() tea.Cmd {
	return nil
}

func (m *sshModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: implement full TUI update loop with MessageBus integration
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.channel.unregisterSession(m.chatID)
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *sshModel) View() string {
	return m.styles.Status.Render(fmt.Sprintf("SSH session: %s | Press q to quit", m.username))
}
