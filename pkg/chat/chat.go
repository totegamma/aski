package chat

import (
	"context"
	"errors"
	"github.com/kznrluk/aski/pkg/config"
	"github.com/kznrluk/aski/pkg/conv"
	"os"
	"os/signal"
	"syscall"
)

type (
	Chat interface {
		Retrieve(conv conv.Conversation, useRest bool) (string, error)
		RetrieveRest(conv conv.Conversation) (string, error)
		RetrieveStream(conv conv.Conversation) (string, error)
	}
)

var (
	ErrCancelled = errors.New("cancelled")
)

func ProvideChat(vendor string, model string, cfg config.Config) (Chat, error) {

	switch vendor {
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, errors.New("OpenAI API key is not set")
		}
		return NewOpenAI(cfg.OpenAIAPIKey), nil
	case "anthropic":
		if cfg.AnthropicAPIKey == "" {
			return nil, errors.New("Anthropic API key is not set")
		}
		return NewAnthropic(cfg.AnthropicAPIKey), nil
	default:
		return nil, errors.New("unsupported vendor: " + vendor)
	}
}

func createCancellableContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT)

		select {
		case <-sigChan:
			println()
			cancel()
		case <-ctx.Done():
		}

		signal.Stop(sigChan)
	}()

	return ctx, cancel
}
