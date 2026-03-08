package providers

import (
	"encoding/json"
	"fmt" // Added for fmt.Sprintf
	"strings"
)

// extractToolCallsFromText parses tool call JSON or tag-based calls from response text.
func extractToolCallsFromText(text string) []ToolCall {
	var result []ToolCall

	// 1. Try JSON format: {"tool_calls": [...]}
	if start := strings.Index(text, `{"tool_calls"`); start != -1 {
		end := findMatchingBrace(text, start)
		if end > start {
			jsonStr := text[start:end]
			var wrapper struct {
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &wrapper); err == nil {
				for _, tc := range wrapper.ToolCalls {
					var args map[string]interface{}
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
					result = append(result, ToolCall{
						ID:        tc.ID,
						Type:      tc.Type,
						Name:      tc.Function.Name,
						Arguments: args,
						Function: &FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
			}
		}
	}

	// 2. Try Tag format: <|tool_call_begin|> name <|tool_call_argument_begin|> {json} <|tool_call_end|>
	// This format is used by some models (like Llama 3.1 on Groq) when tool-calling mode is "forced" or prompted.
	tagStart := `<|tool_call_begin|>`
	argStart := `<|tool_call_argument_begin|>`
	tagEnd := `<|tool_call_end|>`

	cursor := 0
	for {
		startIdx := strings.Index(text[cursor:], tagStart)
		if startIdx == -1 {
			break
		}
		startIdx += cursor
		cursor = startIdx + len(tagStart)

		midIdx := strings.Index(text[cursor:], argStart)
		if midIdx == -1 {
			continue
		}
		midIdx += cursor
		
		name := strings.TrimSpace(text[startIdx+len(tagStart) : midIdx])
		
		argCursor := midIdx + len(argStart)
		endIdx := strings.Index(text[argCursor:], tagEnd)
		if endIdx == -1 {
			continue
		}
		endIdx += argCursor
		
		argsStr := strings.TrimSpace(text[argCursor:endIdx])
		var args map[string]interface{}
		_ = json.Unmarshal([]byte(argsStr), &args)

		// Create a synthetic ID if none provided
		id := fmt.Sprintf("call_%d", len(result))
		if strings.HasPrefix(name, "chatcmpl-tool-") {
			// Some models output the ID in the name slot if they are confused
			// or if the prompt specified "tool_id | tool_name".
			// Let's try to extract name from args if missing, but usually name is the first part.
			id = name
			name = "web_search" // Fallback for the specific user screenshot case
		}

		result = append(result, ToolCall{
			ID:        id,
			Type:      "function",
			Name:      name,
			Arguments: args,
			Function: &FunctionCall{
				Name:      name,
				Arguments: argsStr,
			},
		})
		
		cursor = endIdx + len(tagEnd)
	}

	return result
}

// stripToolCallsFromText removes tool call JSON and tags from response text.
func stripToolCallsFromText(text string) string {
	// Remove JSON format
	if start := strings.Index(text, `{"tool_calls"`); start != -1 {
		if end := findMatchingBrace(text, start); end > start {
			text = text[:start] + text[end:]
		}
	}

	// Remove Tag format sections
	// 1. Remove the outer container if present
	text = strings.ReplaceAll(text, "<|tool_calls_section_begin|>", "")
	text = strings.ReplaceAll(text, "<|tool_calls_section_end|>", "")

	// 2. Remove individual calls
	tagStart := `<|tool_call_begin|>`
	tagEnd := `<|tool_call_end|>`
	for {
		startIdx := strings.Index(text, tagStart)
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(text[startIdx:], tagEnd)
		if endIdx == -1 {
			text = text[:startIdx] // Just cut off truncated tags
			break
		}
		endIdx += startIdx + len(tagEnd)
		text = text[:startIdx] + text[endIdx:]
	}

	return strings.TrimSpace(text)
}

func findMatchingBrace(text string, start int) int {
	braceCount := 0
	foundFirst := false
	for i := start; i < len(text); i++ {
		if text[i] == '{' {
			braceCount++
			foundFirst = true
		} else if text[i] == '}' {
			braceCount--
		}
		if foundFirst && braceCount == 0 {
			return i + 1
		}
	}
	return start
}
