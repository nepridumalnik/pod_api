package models

// ChatResponse is a unified chat completion response model compatible
// with both GigaChat API and OpenAI Chat Completions.
// Fields are optional when they are not provided by a backend.
type ChatResponse struct {
	// ID unique identifier (present in OpenAI; empty for GigaChat)
	ID string `json:"id,omitempty"`

	// Object name of the API object (e.g., "chat.completion")
	Object string `json:"object,omitempty"`

	// Created Unix timestamp
	Created int64 `json:"created,omitempty"`

	// Model name used for generation
	Model string `json:"model,omitempty"`

	// Choices list of hypotheses/messages
	Choices []ChatChoice `json:"choices,omitempty"`

	// Usage token accounting
	Usage *ChatUsage `json:"usage,omitempty"`
}

// ChatChoice represents a single hypothesis in the completion.
type ChatChoice struct {
	Index        int32       `json:"index,omitempty"`
	Message      ChatMessage `json:"message,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// ChatMessage is a detailed message compatible with both APIs.
// Mirrors GigaChat's MessagesRes and OpenAI's chat message fields.
type ChatMessage struct {
	Role             string        `json:"role,omitempty"`
	Content          string        `json:"content,omitempty"`
	Name             string        `json:"name,omitempty"`
	Created          int64         `json:"created,omitempty"`
	FunctionCall     *FunctionCall `json:"function_call,omitempty"`
	FunctionsStateID string        `json:"functions_state_id,omitempty"`

	// ToolCalls are supported by OpenAI. Left empty for GigaChat unless mapped.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// FunctionCall contains function name and arguments.
type FunctionCall struct {
	Name      string                 `json:"name,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCall is provided for OpenAI compatibility. Not used by GigaChat
// but included for clients expecting the field.
type ToolCall struct {
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function map[string]interface{} `json:"function,omitempty"`
}

// ChatUsage aligns with token accounting from both APIs.
type ChatUsage struct {
	PromptTokens          int32 `json:"prompt_tokens,omitempty"`
	CompletionTokens      int32 `json:"completion_tokens,omitempty"`
	TotalTokens           int32 `json:"total_tokens,omitempty"`
	PrecachedPromptTokens int32 `json:"precached_prompt_tokens,omitempty"`
}
