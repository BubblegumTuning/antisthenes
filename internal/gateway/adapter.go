package gateway

import "context"

// MessageEvent represents a normalised incoming message from any platform.
type MessageEvent struct {
	Platform  string
	ChatID    string
	UserID    string
	Text      string
	IsCommand bool
	Raw       interface{} // platform-specific payload if needed
}

// PlatformAdapter defines the contract every messaging platform must implement.
// Adapters are responsible for both inbound and outbound communication.
type PlatformAdapter interface {
	// Name returns the platform identifier (e.g. "telegram", "whatsapp").
	Name() string

	// Start begins listening for incoming messages.
	// Implementations should run their receive loop in a goroutine.
	Start(ctx context.Context) error

	// SendMessage delivers a text message to the given chat/user.
	SendMessage(ctx context.Context, chatID, text string) error

	// Incoming returns a channel of normalised messages from this platform.
	Incoming() <-chan MessageEvent

	// Stop gracefully shuts down the adapter.
	Stop() error
}
