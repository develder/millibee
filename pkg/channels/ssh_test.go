package channels

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
)

func TestNewSSHChannel_Basic(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled: true,
		Address: "127.0.0.1:0",
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}
	if ch.Name() != "ssh" {
		t.Errorf("Name() = %q, want 'ssh'", ch.Name())
	}
	if ch.IsRunning() {
		t.Error("should not be running before Start()")
	}
}

func TestSSHChannel_IsAllowed_EmptyList(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	if !ch.IsAllowed("anyone") {
		t.Error("empty allowlist should allow anyone")
	}
}

func TestSSHChannel_IsAllowed_WithList(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled:   true,
		Address:   "127.0.0.1:0",
		AllowFrom: []string{"alice", "bob"},
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	if !ch.IsAllowed("alice") {
		t.Error("alice should be allowed")
	}
	if !ch.IsAllowed("bob") {
		t.Error("bob should be allowed")
	}
	if ch.IsAllowed("eve") {
		t.Error("eve should not be allowed")
	}
}

func TestSSHChannel_HandleMessage_PublishesToBus(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	ch.HandleMessage("alice", "ssh:alice", "hello", nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg, ok := msgBus.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("expected inbound message from bus")
	}
	if msg.Channel != "ssh" {
		t.Errorf("Channel = %q, want 'ssh'", msg.Channel)
	}
	if msg.SenderID != "alice" {
		t.Errorf("SenderID = %q, want 'alice'", msg.SenderID)
	}
	if msg.ChatID != "ssh:alice" {
		t.Errorf("ChatID = %q, want 'ssh:alice'", msg.ChatID)
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want 'hello'", msg.Content)
	}
}

func TestSSHChannel_Send_NoSession(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	err = ch.Send(context.Background(), bus.OutboundMessage{
		Channel: "ssh",
		ChatID:  "ssh:nobody",
		Content: "hello",
	})
	if err == nil {
		t.Error("Send() to non-existent session should return error")
	}
}

func TestSSHChannel_Send_WithSession(t *testing.T) {
	cfg := config.SSHConfig{Enabled: true, Address: "127.0.0.1:0"}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	// Register a fake session
	outChan := make(chan string, 10)
	ch.registerSession("ssh:alice", outChan)

	err = ch.Send(context.Background(), bus.OutboundMessage{
		Channel: "ssh",
		ChatID:  "ssh:alice",
		Content: "response text",
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	select {
	case msg := <-outChan:
		if msg != "response text" {
			t.Errorf("outChan received %q, want 'response text'", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message on outChan")
	}

	// Cleanup
	ch.unregisterSession("ssh:alice")
}

func TestSSHChannel_StartStop(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled: true,
		Address: "127.0.0.1:0", // random port
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	ctx := context.Background()
	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if !ch.IsRunning() {
		t.Error("should be running after Start()")
	}

	// Verify something is listening
	addr := ch.ListenAddr()
	if addr == "" {
		t.Fatal("ListenAddr() should return address after Start()")
	}

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("could not connect to SSH server at %s: %v", addr, err)
	}
	conn.Close()

	// Stop
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ch.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if ch.IsRunning() {
		t.Error("should not be running after Stop()")
	}

	// Verify port is closed (may take a moment)
	time.Sleep(100 * time.Millisecond)
	conn, err = net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Error("should not be able to connect after Stop()")
	}
}

func TestSSHChannel_PasswordAuth(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled:  true,
		Address:  "127.0.0.1:0",
		Password: "secret123",
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	// Verify password validator is set
	if !ch.hasPasswordAuth() {
		t.Error("channel should have password auth when password is configured")
	}

	// Verify validator accepts correct password
	if !ch.validatePassword("anyuser", "secret123") {
		t.Error("correct password should be accepted")
	}

	// Verify validator rejects wrong password
	if ch.validatePassword("anyuser", "wrong") {
		t.Error("wrong password should be rejected")
	}
}

func TestSSHChannel_NoPasswordAuth(t *testing.T) {
	cfg := config.SSHConfig{
		Enabled: true,
		Address: "127.0.0.1:0",
	}
	msgBus := bus.NewMessageBus()

	ch, err := NewSSHChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewSSHChannel() error: %v", err)
	}

	if ch.hasPasswordAuth() {
		t.Error("channel should not have password auth when no password configured")
	}
}

// Helper to get a free port for testing
func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

func freeAddr(t *testing.T) string {
	t.Helper()
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("getFreePort: %v", err)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}
