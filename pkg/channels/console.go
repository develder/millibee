package channels

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/develder/millibee/pkg/bus"
	"github.com/develder/millibee/pkg/config"
	"github.com/develder/millibee/pkg/logger"
)

// ConsoleChannel implements Channel for interactive stdin/stdout communication
// in gateway mode. Unlike the internal "cli" channel, "console" participates
// in outbound message dispatch.
type ConsoleChannel struct {
	*BaseChannel
	reader io.Reader
	writer io.Writer
	cancel context.CancelFunc
}

// NewConsoleChannel creates a console channel that reads from stdin and writes to stdout.
func NewConsoleChannel(cfg config.ConsoleConfig, msgBus *bus.MessageBus) (*ConsoleChannel, error) {
	base := NewBaseChannel("console", cfg, msgBus, nil)
	return &ConsoleChannel{
		BaseChannel: base,
		reader:      os.Stdin,
		writer:      os.Stdout,
	}, nil
}

// Start begins reading lines from the reader in a background goroutine.
func (c *ConsoleChannel) Start(ctx context.Context) error {
	readCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.setRunning(true)

	logger.InfoC("console", "Console channel started")

	go c.readLoop(readCtx)
	return nil
}

func (c *ConsoleChannel) readLoop(ctx context.Context) {
	scanner := bufio.NewScanner(c.reader)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					logger.ErrorCF("console", "Error reading stdin", map[string]any{
						"error": err.Error(),
					})
				}
				return
			}

			input := strings.TrimSpace(scanner.Text())
			if input == "" {
				continue
			}

			c.HandleMessage("console", "console", input, nil, nil)
		}
	}
}

// Stop halts the stdin reading goroutine.
func (c *ConsoleChannel) Stop(ctx context.Context) error {
	logger.InfoC("console", "Stopping console channel")
	if c.cancel != nil {
		c.cancel()
	}
	c.setRunning(false)
	return nil
}

// Send writes the agent's response to the writer (stdout by default).
func (c *ConsoleChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("console channel not running")
	}

	fmt.Fprintf(c.writer, "\n%s\n\n", msg.Content)
	return nil
}
