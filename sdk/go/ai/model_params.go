package ai

import (
	"encoding/json"
	"strings"
)

// needsMaxCompletionTokens returns true if the model requires
// max_completion_tokens instead of max_tokens. This applies to OpenAI's
// newer model families (o1, o3, gpt-4o, etc.) which dropped support for
// the legacy max_tokens parameter.
//
// Reference: https://platform.openai.com/docs/api-reference/chat/create
func needsMaxCompletionTokens(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))

	// Strip provider prefix if present (e.g. "openai/gpt-4o" → "gpt-4o")
	if idx := strings.LastIndex(m, "/"); idx >= 0 {
		m = m[idx+1:]
	}

	// o1, o3, o4 series always use max_completion_tokens
	if strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") {
		return true
	}

	// gpt-4o and gpt-4o-mini series use max_completion_tokens
	if strings.HasPrefix(m, "gpt-4o") {
		return true
	}

	// gpt-4.1, gpt-4.5 etc. (newer dot-release models)
	if strings.HasPrefix(m, "gpt-4.") {
		return true
	}

	return false
}

// isOpenAICompatible returns true if the base URL points to OpenAI or an
// OpenAI-compatible endpoint (not Anthropic, Cohere, etc.).
func isOpenAICompatible(baseURL string) bool {
	lower := strings.ToLower(baseURL)

	// Known non-OpenAI providers (not OpenAI-compatible chat/completions).
	if strings.Contains(lower, "anthropic") || strings.Contains(lower, "cohere") {
		return false
	}

	// OpenAI's own endpoint
	if strings.Contains(lower, "openai.com") {
		return true
	}
	// OpenRouter proxies to OpenAI models
	if strings.Contains(lower, "openrouter.ai") {
		return true
	}
	// Azure OpenAI
	if strings.Contains(lower, "openai.azure.com") {
		return true
	}
	// For unknown/custom endpoints, assume OpenAI-compatible since that's
	// the most common case for the chat/completions API shape.
	return true
}

// marshalRequest serializes the request, applying provider-specific parameter
// rewrites. For OpenAI-compatible endpoints with newer models, max_tokens is
// rewritten to max_completion_tokens.
func (c *Client) marshalRequest(req *Request) ([]byte, error) {
	model := req.Model
	if model == "" {
		model = c.config.Model
	}

	// If the model needs max_completion_tokens and we have a max_tokens value,
	// serialize with the rewritten field name.
	if req.MaxTokens != nil && needsMaxCompletionTokens(model) && isOpenAICompatible(c.config.BaseURL) {
		return marshalWithMaxCompletionTokens(req)
	}

	return json.Marshal(req)
}

// marshalWithMaxCompletionTokens serializes the request with max_completion_tokens
// instead of max_tokens. We use a shadow struct to avoid modifying the original.
func marshalWithMaxCompletionTokens(req *Request) ([]byte, error) {
	type requestAlias Request

	wire := struct {
		*requestAlias
		MaxTokens           *int `json:"max_tokens,omitempty"`
		MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`
	}{
		requestAlias:        (*requestAlias)(req),
		MaxTokens:           nil, // suppress max_tokens
		MaxCompletionTokens: req.MaxTokens,
	}

	return json.Marshal(wire)
}
