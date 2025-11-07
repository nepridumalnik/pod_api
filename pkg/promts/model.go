package prompts

// Role соответствует стандарту chat-моделей.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message описывает одно сообщение диалога.
// Структура подходит для OpenAI, DeepSeek и Gigachat.
type Message struct {
	Role        Role       `json:"role"`
	Content     string     `json:"content,omitempty"`
	Name        string     `json:"name,omitempty"`
	ContentType string     `json:"content_type,omitempty"` // "text", "json", "image_url", и т.д.
	ToolCalls   []ToolCall `json:"tool_calls,omitempty"`   // OpenAI/DeepSeek tool calls
	ToolCallID  string     `json:"tool_call_id,omitempty"` // ответ конкретному вызову инструмента
	Attachments any        `json:"attachments,omitempty"`  // Gigachat: изображения, файлы
}

// ToolCall описывает единичный вызов инструмента (для OpenAI/DeepSeek API).
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction — вызываемая функция для ToolCall.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Prompt — контейнер сообщений, может использоваться для хранения шаблонов.
type Prompt struct {
	ID       string    `json:"id"`
	Messages []Message `json:"messages"`
	Meta     *Meta     `json:"meta,omitempty"`
}

// Meta — необязательные метаданные.
type Meta struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// ChatRequest — универсальная структура запроса для OpenAI/DeepSeek/Gigachat.
type ChatRequest struct {
	Model            string     `json:"model"`
	Messages         []Message  `json:"messages"`
	Stream           bool       `json:"stream,omitempty"`
	MaxTokens        int        `json:"max_tokens,omitempty"`
	Temperature      float32    `json:"temperature,omitempty"`
	TopP             float32    `json:"top_p,omitempty"`
	FrequencyPenalty float32    `json:"frequency_penalty,omitempty"`
	PresencePenalty  float32    `json:"presence_penalty,omitempty"`
	Tools            []ToolSpec `json:"tools,omitempty"` // поддержка OpenAI function calling
}

// ToolSpec — описание функции/инструмента для модели.
type ToolSpec struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ResponseMessage — универсальный формат для полученного сообщения.
type ResponseMessage struct {
	Role        Role       `json:"role"`
	Content     string     `json:"content,omitempty"`
	ToolCalls   []ToolCall `json:"tool_calls,omitempty"`
	Attachments any        `json:"attachments,omitempty"`
}

// ChatChoice — элемент списка ответов.
type ChatChoice struct {
	Index        int              `json:"index"`
	Message      ResponseMessage  `json:"message"`
	FinishReason string           `json:"finish_reason"`
	Delta        *ResponseMessage `json:"delta,omitempty"` // для stream-ответов
}

// ChatResponse — общий формат для OpenAI, DeepSeek и Gigachat.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"` // Gigachat и OpenAI возвращают токен-статистику
}

// Usage — информация о количестве использованных токенов.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}
