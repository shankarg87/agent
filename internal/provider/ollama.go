package provider

import (
	"context"
	"fmt"

	"github.com/shankarg87/agent/internal/config"
)

// OllamaProvider implements Provider for Ollama models
// TODO: Implement full Ollama support
type OllamaProvider struct {
	endpoint string
	model    string
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(cfg config.ModelConfig) (*OllamaProvider, error) {
	return nil, fmt.Errorf("ollama provider not yet implemented")
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) Model() string {
	return p.model
}

func (p *OllamaProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return nil, fmt.Errorf("ollama provider not yet implemented")
}

func (p *OllamaProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	return nil, fmt.Errorf("ollama provider not yet implemented")
}
