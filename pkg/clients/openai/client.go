package openai

import (
	"context"
	"fmt"
	"strings"

	"pod_api/pkg/models"
	"pod_api/pkg/prompting"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Client struct {
	client openai.Client
	model  string
}

func isModelInList(model string, models []openai.Model) bool {
	for i := range models {
		if models[i].ID == model {
			return true
		}
	}

	return false
}

func NewClient(key string, url string, model string) (*Client, error) {
	client := openai.NewClient(option.WithAPIKey(key), option.WithBaseURL(url))

	// Test connectivity by listing models
	modelList, err := client.Models.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	if !isModelInList(model, modelList.Data) {
		return nil, fmt.Errorf("such model does not exists: %s", model)
	}

	return &Client{
		client: client,
		model:  model,
	}, nil
}

func (c *Client) makePromtParams(message string, imageURL string) openai.ChatCompletionNewParams {
	// var jsonFmt constant.JSONObject
	return openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(c.model),
		MaxTokens: openai.Int(50000),
		// ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
		// 	OfJSONObject: &shared.ResponseFormatJSONObjectParam{
		// 		Type: jsonFmt.Default(),
		// 	},
		// },
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
							{OfText: &openai.ChatCompletionContentPartTextParam{
								Text: message,
							}},
							{OfImageURL: &openai.ChatCompletionContentPartImageParam{
								ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
									URL:    imageURL,
									Detail: "auto",
								},
							}},
						},
					},
				},
			},
		},
	}
}

func (c *Client) SendImage(message string, imageURL string) (*models.ChatResponse, error) {
	params := c.makePromtParams(message, imageURL)
	response, err := c.client.Chat.Completions.New(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}

	out := &models.ChatResponse{
		ID:      response.ID,
		Object:  string(response.Object),
		Created: response.Created,
		Model:   response.Model,
		Usage: &models.ChatUsage{
			PromptTokens:     int32(response.Usage.PromptTokens),
			CompletionTokens: int32(response.Usage.CompletionTokens),
			TotalTokens:      int32(response.Usage.TotalTokens),
		},
	}

	for _, choice := range response.Choices {
		message := models.ChatMessage{
			Content: trimMessage(choice.Message.Content),
			Role:    string(choice.Message.Role),
		}
		out.Choices = append(out.Choices, models.ChatChoice{
			Index:        int32(choice.Index),
			FinishReason: choice.FinishReason,
			Message:      message,
		})
	}

	return out, nil
}

func trimMessage(message string) string {
	return strings.TrimPrefix(strings.TrimSuffix(message, "\n```"), "```json\n")
}
