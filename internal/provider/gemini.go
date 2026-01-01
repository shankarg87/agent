package provider

import (
	"context"
	"fmt"

	"github.com/shankgan/agent/internal/config"
)

// GeminiProvider implements Provider for Google Gemini models
// TODO: Implement full Gemini support
type GeminiProvider struct {
	apiKey string
	model  string
}

// NewGeminiProvider creates a new Gemini provider
func NewGeminiProvider(cfg config.ModelConfig) (*GeminiProvider, error) {
	return nil, fmt.Errorf("gemini provider not yet implemented")
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

func (p *GeminiProvider) Model() string {
	return p.model
}

func (p *GeminiProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return nil, fmt.Errorf("gemini provider not yet implemented")
}

func (p *GeminiProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	return nil, fmt.Errorf("gemini provider not yet implemented")
}
