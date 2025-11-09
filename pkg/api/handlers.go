package api

import (
	"context"
	"errors"

	apigen "pod_api/pkg/apigen/openapi"
	"pod_api/pkg/models"
)

type TextModel interface {
	// SendMessage sends user text to the model and returns a unified
	// chat response compatible with GigaChat/OpenAI along with an error.
	SendMessage(message string) (*models.ChatResponse, error)
}

type ImageModel interface {
	SendImage(message string, url string) (*models.ChatResponse, error)
}

// Handlers implements apigen.StrictServerInterface.
type Handlers struct {
	text  TextModel
	image ImageModel
}

// NewHandlers constructs Handlers with provided models.
func NewHandlers(text TextModel, image ImageModel) (*Handlers, error) {
	if text == nil {
		return nil, errors.New("text model should not be nil")
	}
	if image == nil {
		return nil, errors.New("image model should not be nil")
	}
	return &Handlers{text: text, image: image}, nil
}

// RespondText handles POST /api/v1/respond/text
func (h *Handlers) RespondText(ctx context.Context, request apigen.RespondTextRequestObject) (apigen.RespondTextResponseObject, error) {
	if request.Body == nil {
		return apigen.RespondText400JSONResponse{Error: "bad_request"}, nil
	}

	response, err := h.text.SendMessage(request.Body.Text)
	if err != nil {
		return nil, err
	}

	// Map first assistant message to the public response shape.
	var items []apigen.ResponseItem
	for i := range response.Choices {
		if response.Choices[i].Message.Content != "" {
			items = append(items, apigen.ResponseItem{
				Description: response.Choices[i].Message.Content,
			})
		}
	}

	return apigen.RespondText200JSONResponse{Items: items}, nil
}

// RespondTextImage handles POST /api/v1/respond/text-image
func (h *Handlers) RespondTextImage(ctx context.Context, request apigen.RespondTextImageRequestObject) (apigen.RespondTextImageResponseObject, error) {
	if request.Body == nil {
		return apigen.RespondTextImage400JSONResponse{Error: "bad_request"}, nil
	}

	response, err := h.image.SendImage(request.Body.Text, request.Body.Image)
	if err != nil {
		return nil, err
	}

	// Маппинг ChatResponse -> CommonResponse
	var items []apigen.ResponseItem
	for _, choice := range response.Choices {
		message := choice.Message
		if message.Content != "" {
			items = append(items, apigen.ResponseItem{
				Name:              response.Model,
				Description:       message.Content,
				MainImageUrl:      request.Body.Image,
				CarouselImageUrls: []string{request.Body.Image},
			})
		}
	}

	// Если модель ничего не вернула - вернуть хотя бы пустой объект
	if len(items) == 0 {
		items = []apigen.ResponseItem{{
			Name:              response.Model,
			Description:       "(empty response)",
			MainImageUrl:      request.Body.Image,
			CarouselImageUrls: []string{request.Body.Image},
		}}
	}

	return apigen.RespondTextImage200JSONResponse{Items: items}, nil
}
