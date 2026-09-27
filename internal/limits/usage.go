package limits

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
)

// TokenUsage holds extracted actual token usage metrics.
type TokenUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// EstimateTokens calculates an upfront estimate of tokens required for a request.
// Heuristic: ~4 characters per token for input text + max_tokens for output.
func EstimateTokens(bodyBytes []byte, defaultMaxTokens int64) int64 {
	if defaultMaxTokens <= 0 {
		defaultMaxTokens = 1000
	}

	if len(bodyBytes) == 0 {
		return defaultMaxTokens
	}

	// Use json.RawMessage to avoid parsing deep conversation trees into arbitrary maps/slices
	var payload struct {
		Prompt              json.RawMessage `json:"prompt"`
		Messages            json.RawMessage `json:"messages"`
		MaxTokens           int64           `json:"max_tokens"`
		MaxCompletionTokens int64           `json:"max_completion_tokens"`
	}

	inputChars := 0
	maxTokens := defaultMaxTokens

	if err := json.Unmarshal(bodyBytes, &payload); err == nil {
		if payload.MaxTokens > 0 {
			maxTokens = payload.MaxTokens
		} else if payload.MaxCompletionTokens > 0 {
			maxTokens = payload.MaxCompletionTokens
		}

		if len(payload.Prompt) > 0 {
			inputChars += len(payload.Prompt)
		}
		if len(payload.Messages) > 0 {
			inputChars += len(payload.Messages)
		}
	} else {
		// Fallback: estimate based on total body length
		inputChars = len(bodyBytes)
	}

	// ~4 characters per token heuristic
	inputTokens := int64(math.Ceil(float64(inputChars) / 4.0))
	if inputTokens < 1 {
		inputTokens = 1
	}

	return inputTokens + maxTokens
}

// Target struct for single-pass unmarshaling (OpenAI, Anthropic, Gemini)
type targetedUsageResponse struct {
	Usage *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		TotalTokens      int64 `json:"total_tokens"`
		InputTokens      int64 `json:"input_tokens"`
		OutputTokens     int64 `json:"output_tokens"`
	} `json:"usage"`
	UsageMetadata *struct {
		PromptTokenCount     int64 `json:"promptTokenCount"`
		CandidatesTokenCount int64 `json:"candidatesTokenCount"`
		TotalTokenCount      int64 `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// ParseUsageFromBody parses token usage from standard LLM response payloads.
// Supports:
// - OpenAI: usage.prompt_tokens, usage.completion_tokens, usage.total_tokens
// - Anthropic: usage.input_tokens, usage.output_tokens
// - Gemini: usageMetadata.promptTokenCount, usageMetadata.candidatesTokenCount
func ParseUsageFromBody(bodyBytes []byte) (*TokenUsage, bool) {
	if len(bodyBytes) == 0 {
		return nil, false
	}

	// Fast path: targeted struct unmarshal avoiding generic map[string]any allocations (P5)
	var target targetedUsageResponse
	if err := json.Unmarshal(bodyBytes, &target); err == nil {
		if target.Usage != nil {
			prompt := target.Usage.PromptTokens
			if prompt == 0 {
				prompt = target.Usage.InputTokens
			}
			completion := target.Usage.CompletionTokens
			if completion == 0 {
				completion = target.Usage.OutputTokens
			}
			total := target.Usage.TotalTokens
			if total == 0 {
				total = prompt + completion
			}
			if total > 0 || prompt > 0 || completion > 0 {
				return &TokenUsage{
					PromptTokens:     prompt,
					CompletionTokens: completion,
					TotalTokens:      total,
				}, true
			}
		}

		if target.UsageMetadata != nil {
			prompt := target.UsageMetadata.PromptTokenCount
			completion := target.UsageMetadata.CandidatesTokenCount
			total := target.UsageMetadata.TotalTokenCount
			if total == 0 {
				total = prompt + completion
			}
			if total > 0 {
				return &TokenUsage{
					PromptTokens:     prompt,
					CompletionTokens: completion,
					TotalTokens:      total,
				}, true
			}
		}
	}

	// Slow fallback: generic unmarshal for non-standard provider structures
	var root map[string]any
	if err := json.Unmarshal(bodyBytes, &root); err != nil {
		return nil, false
	}

	if uVal, ok := root["usage"]; ok {
		if uMap, isMap := uVal.(map[string]any); isMap {
			return extractFromMap(uMap)
		}
	}

	if uVal, ok := root["usageMetadata"]; ok {
		if uMap, isMap := uVal.(map[string]any); isMap {
			normalized := normalizeKeys(uMap)
			prompt := getInt64(normalized, "prompttokencount")
			completion := getInt64(normalized, "candidatestokencount")
			total := getInt64(normalized, "totaltokencount")
			if total == 0 {
				total = prompt + completion
			}
			if total > 0 {
				return &TokenUsage{
					PromptTokens:     prompt,
					CompletionTokens: completion,
					TotalTokens:      total,
				}, true
			}
		}
	}

	return nil, false
}

// ParseUsageFromChunk inspects an SSE chunk for final usage statistics.
func ParseUsageFromChunk(chunk []byte) (*TokenUsage, bool) {
	lines := bytes.Split(chunk, []byte("\n"))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("data:")) {
			dataContent := bytes.TrimSpace(trimmed[5:])
			if bytes.Equal(dataContent, []byte("[DONE]")) {
				continue
			}
			if usage, ok := ParseUsageFromBody(dataContent); ok {
				return usage, true
			}
		}
	}
	return nil, false
}

func normalizeKeys(m map[string]any) map[string]any {
	norm := make(map[string]any, len(m))
	for k, v := range m {
		norm[strings.ToLower(k)] = v
	}
	return norm
}

func extractFromMap(m map[string]any) (*TokenUsage, bool) {
	// Normalize keys once upfront (P9)
	normalized := normalizeKeys(m)

	prompt := getInt64(normalized, "prompt_tokens")
	if prompt == 0 {
		prompt = getInt64(normalized, "input_tokens")
	}

	completion := getInt64(normalized, "completion_tokens")
	if completion == 0 {
		completion = getInt64(normalized, "output_tokens")
	}

	total := getInt64(normalized, "total_tokens")
	if total == 0 {
		total = prompt + completion
	}

	if total > 0 || prompt > 0 || completion > 0 {
		return &TokenUsage{
			PromptTokens:     prompt,
			CompletionTokens: completion,
			TotalTokens:      total,
		}, true
	}

	return nil, false
}

func getInt64(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}

	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case json.Number:
		i, _ := val.Int64()
		return i
	default:
		return 0
	}
}
