package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"pod_api/pkg/models"
	"pod_api/pkg/prompting"
)

type Client struct {
	client openai.Client
}

func NewClient(key string, url string) (*Client, error) {
	client := openai.NewClient(option.WithAPIKey(key), option.WithBaseURL(url))

	// Smoke-test connectivity by listing models
	_, err := client.Models.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	return &Client{client: client}, nil
}

// SendImage consumes a multipart form reader with fields:
// - text: prompt text (optional if usersMessage provided)
// - image: image binary (required)
// It sends a text+image user message to an OpenAI vision-capable model and
// returns a unified chat response structure.
func (c *Client) SendImage(usersMessage string, reader multipart.Reader) (*models.ChatResponse, error) {
	// Parse multipart parts
	var (
		prompt         = usersMessage
		imageBytes     []byte
		imageMediaType string
	)

	r := &reader
	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("multipart read failed: %w", err)
		}
		name := part.FormName()
		switch name {
		case "text":
			if prompt == "" {
				b, _ := io.ReadAll(part)
				prompt = string(b)
			} else {
				// drain
				_, _ = io.Copy(io.Discard, part)
			}
		case "image":
			b, _ := io.ReadAll(part)
			if len(b) > 0 {
				imageBytes = b
				imageMediaType = part.Header.Get("Content-Type")
				if imageMediaType == "" {
					imageMediaType = http.DetectContentType(b)
				}
			}
		default:
			// unknown field, drain
			_, _ = io.Copy(io.Discard, part)
		}
	}

	if len(imageBytes) == 0 {
		return nil, errors.New("image field is required")
	}
	if prompt == "" {
		// Provide a minimal default prompt if not specified
		prompt = "Опиши изображение"
	}

	// Build data URI for inline image
	if imageMediaType == "" {
		imageMediaType = "image/png"
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", imageMediaType, base64.StdEncoding.EncodeToString(imageBytes))

	// Construct a chat.completions request with text + image content
	// Use a vision-capable default model.
	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String(prompting.SystemPrompt()),
					},
				},
			},
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfArrayOfContentParts: []openai.ChatCompletionContentPartUnionParam{
							{OfText: &openai.ChatCompletionContentPartTextParam{Text: prompt}},
							{OfImageURL: &openai.ChatCompletionContentPartImageParam{
								ImageURL: openai.ChatCompletionContentPartImageImageURLParam{URL: dataURI},
							}},
						},
					},
				},
			},
		},
		// Optionally limit tokens if desired; omitted for now to use model defaults.
	}

	// Execute request
	resp, err := c.client.Chat.Completions.New(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("openai vision request failed: %w", err)
	}

	// Map OpenAI response to unified models.ChatResponse
	out := &models.ChatResponse{
		ID:      resp.ID,
		Object:  string(resp.Object),
		Created: resp.Created,
		Model:   resp.Model,
	}

	// Usage mapping
	out.Usage = &models.ChatUsage{}
	out.Usage.PromptTokens = int32(resp.Usage.PromptTokens)
	out.Usage.CompletionTokens = int32(resp.Usage.CompletionTokens)
	out.Usage.TotalTokens = int32(resp.Usage.TotalTokens)

	// Choices/messages mapping (content + role + finish_reason)
	for _, ch := range resp.Choices {
		choice := models.ChatChoice{}
		choice.Index = int32(ch.Index)
		choice.FinishReason = ch.FinishReason

		m := models.ChatMessage{}
		m.Content = ch.Message.Content
		m.Role = string(ch.Message.Role)

		// Map deprecated function_call if present: parse arguments JSON if possible
		if ch.Message.FunctionCall.Name != "" || ch.Message.FunctionCall.Arguments != "" {
			fc := &models.FunctionCall{Name: ch.Message.FunctionCall.Name}
			if ch.Message.FunctionCall.Arguments != "" {
				var args map[string]any
				if err := json.Unmarshal([]byte(ch.Message.FunctionCall.Arguments), &args); err == nil {
					fc.Arguments = args
				}
			}
			m.FunctionCall = fc
		}

		choice.Message = m
		out.Choices = append(out.Choices, choice)
	}

	return out, nil
}
