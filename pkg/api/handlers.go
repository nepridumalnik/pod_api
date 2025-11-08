package api

import (
	"context"
	"errors"
	"mime/multipart"

	apigen "pod_api/pkg/apigen/openapi"
	"pod_api/pkg/models"
)

type TextModel interface {
	// SendMessage sends user text to the model and returns a unified
	// chat response compatible with GigaChat/OpenAI along with an error.
	SendMessage(usersMessage string) (*models.ChatResponse, error)
}

type ImageModel interface {
	SendImage(usersMessage string, reader multipart.Reader) error
}

// Handlers implements apigen.StrictServerInterface.
type Handlers struct {
	text TextModel
	img  ImageModel
}

// NewHandlers constructs Handlers with provided models.
func NewHandlers(text TextModel, img ImageModel) (*Handlers, error) {
	if text == nil {
		return nil, errors.New("text model should not be nil")
	}
	return &Handlers{text: text, img: img}, nil
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
	return apigen.RespondTextImage200JSONResponse{Items: []apigen.ResponseItem{}}, nil
}
