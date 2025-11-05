package api

import (
	"context"

	"pod_api/pkg/apigen"
)

// Handlers implements apigen.StrictServerInterface with default empty responses.
type Handlers struct{}

// RespondText handles POST /api/v1/respond/text
func (h *Handlers) RespondText(ctx context.Context, request apigen.RespondTextRequestObject) (apigen.RespondTextResponseObject, error) {
	// Return an empty CommonResponse by default
	return apigen.RespondText200JSONResponse{
		Items: []apigen.ResponseItem{},
	}, nil
}

// RespondTextImage handles POST /api/v1/respond/text-image
func (h *Handlers) RespondTextImage(ctx context.Context, request apigen.RespondTextImageRequestObject) (apigen.RespondTextImageResponseObject, error) {
	// Return an empty CommonResponse by default
	return apigen.RespondTextImage200JSONResponse{
		Items: []apigen.ResponseItem{},
	}, nil
}
