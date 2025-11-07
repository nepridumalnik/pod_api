package api

import (
	"context"
	"mime/multipart"

	apigen "pod_api/pkg/apigen/openapi"
)

type TextModel interface {
	SendMessage(usersMessage string) error
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
func NewHandlers(text TextModel, img ImageModel) *Handlers {
	return &Handlers{text: text, img: img}
}

// RespondText handles POST /api/v1/respond/text
func (h *Handlers) RespondText(ctx context.Context, request apigen.RespondTextRequestObject) (apigen.RespondTextResponseObject, error) {
	if h.text != nil && request.Body != nil {
		_ = h.text.SendMessage(request.Body.Text)
	}
	return apigen.RespondText200JSONResponse{Items: []apigen.ResponseItem{}}, nil
}

// RespondTextImage handles POST /api/v1/respond/text-image
func (h *Handlers) RespondTextImage(ctx context.Context, request apigen.RespondTextImageRequestObject) (apigen.RespondTextImageResponseObject, error) {
	return apigen.RespondTextImage200JSONResponse{Items: []apigen.ResponseItem{}}, nil
}
