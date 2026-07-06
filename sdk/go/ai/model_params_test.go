package ai

import (
	"encoding/json"
	"testing"
)

func TestNeedsMaxCompletionTokens(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		// o1 series
		{"o1-mini", true},
		{"o1-preview", true},
		{"o1", true},
		// o3 series
		{"o3-mini", true},
		{"o3", true},
		// o4 series
		{"o4-mini", true},
		// gpt-4o series
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"gpt-4o-2024-05-13", true},
		// gpt-4.x dot releases
		{"gpt-4.1", true},
		{"gpt-4.5-preview", true},
		// With provider prefix
		{"openai/gpt-4o", true},
		{"openai/o1-mini", true},
		{"openrouter/openai/gpt-4o-mini", true},
		// Case insensitive
		{"GPT-4O", true},
		{"O1-Mini", true},
		// Legacy models — should NOT rewrite
		{"gpt-4", false},
		{"gpt-4-turbo", false},
		{"gpt-3.5-turbo", false},
		{"gpt-4-0613", false},
		// Non-OpenAI models
		{"claude-3-opus", false},
		{"mistral-large", false},
		{"llama-3-70b", false},
		// Empty
		{"", false},
	}

	for _, tt := range tests {
		got := needsMaxCompletionTokens(tt.model)
		if got != tt.expected {
			t.Errorf("needsMaxCompletionTokens(%q) = %v, want %v", tt.model, got, tt.expected)
		}
	}
}

func TestMarshalRequest_RewritesMaxTokensForNewModels(t *testing.T) {
	maxTokens := 1000
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.Model = "gpt-4o"

	client, _ := NewClient(cfg)

	req := &Request{
		Model:     "gpt-4o",
		MaxTokens: &maxTokens,
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
		},
	}

	body, err := client.marshalRequest(req)
	if err != nil {
		t.Fatalf("marshalRequest error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := raw["max_tokens"]; ok {
		t.Error("expected max_tokens to be absent for gpt-4o model")
	}
	if val, ok := raw["max_completion_tokens"]; !ok {
		t.Error("expected max_completion_tokens to be present for gpt-4o model")
	} else if int(val.(float64)) != 1000 {
		t.Errorf("expected max_completion_tokens=1000, got %v", val)
	}
}

func TestMarshalRequest_KeepsMaxTokensForLegacyModels(t *testing.T) {
	maxTokens := 2000
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.Model = "gpt-3.5-turbo"

	client, _ := NewClient(cfg)

	req := &Request{
		Model:     "gpt-3.5-turbo",
		MaxTokens: &maxTokens,
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
		},
	}

	body, err := client.marshalRequest(req)
	if err != nil {
		t.Fatalf("marshalRequest error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := raw["max_completion_tokens"]; ok {
		t.Error("expected max_completion_tokens to be absent for gpt-3.5-turbo model")
	}
	if val, ok := raw["max_tokens"]; !ok {
		t.Error("expected max_tokens to be present for gpt-3.5-turbo model")
	} else if int(val.(float64)) != 2000 {
		t.Errorf("expected max_tokens=2000, got %v", val)
	}
}

func TestMarshalRequest_O1MiniUsesMaxCompletionTokens(t *testing.T) {
	maxTokens := 500
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.Model = "o1-mini"

	client, _ := NewClient(cfg)

	req := &Request{
		Model:     "o1-mini",
		MaxTokens: &maxTokens,
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
		},
	}

	body, err := client.marshalRequest(req)
	if err != nil {
		t.Fatalf("marshalRequest error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := raw["max_tokens"]; ok {
		t.Error("expected max_tokens to be absent for o1-mini")
	}
	if val, ok := raw["max_completion_tokens"]; !ok {
		t.Error("expected max_completion_tokens to be present for o1-mini")
	} else if int(val.(float64)) != 500 {
		t.Errorf("expected max_completion_tokens=500, got %v", val)
	}
}

func TestMarshalRequest_NilMaxTokensOmitted(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.Model = "gpt-4o"

	client, _ := NewClient(cfg)

	req := &Request{
		Model:     "gpt-4o",
		MaxTokens: nil,
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
		},
	}

	body, err := client.marshalRequest(req)
	if err != nil {
		t.Fatalf("marshalRequest error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := raw["max_tokens"]; ok {
		t.Error("expected max_tokens to be absent when nil")
	}
	if _, ok := raw["max_completion_tokens"]; ok {
		t.Error("expected max_completion_tokens to be absent when MaxTokens is nil")
	}
}

func TestMarshalRequest_UsesRequestModelOverClientModel(t *testing.T) {
	maxTokens := 800
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"
	cfg.Model = "gpt-3.5-turbo" // client default is legacy

	client, _ := NewClient(cfg)

	req := &Request{
		Model:     "o1-preview", // request overrides to new model
		MaxTokens: &maxTokens,
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
		},
	}

	body, err := client.marshalRequest(req)
	if err != nil {
		t.Fatalf("marshalRequest error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := raw["max_tokens"]; ok {
		t.Error("expected max_tokens to be absent for o1-preview")
	}
	if _, ok := raw["max_completion_tokens"]; !ok {
		t.Error("expected max_completion_tokens to be present for o1-preview")
	}
}
