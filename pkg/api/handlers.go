package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	apigen "pod_api/pkg/apigen/openapi"
	"pod_api/pkg/models"
	imagerepo "pod_api/pkg/repository/image"
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
	text            TextModel
	image           ImageModel
	imageRepository imagerepo.ImageRepository
	baseURL         string
	imageTTL        time.Duration
}

// NewHandlers constructs Handlers with provided models and dependencies.
func NewHandlers(text TextModel, image ImageModel, imageRepository imagerepo.ImageRepository, baseURL string, imageTTL time.Duration) (*Handlers, error) {
	if text == nil {
		return nil, errors.New("text model should not be nil")
	}
	if image == nil {
		return nil, errors.New("image model should not be nil")
	}
	if imageRepository == nil {
		return nil, errors.New("image repository should not be nil")
	}
	return &Handlers{
		text:            text,
		image:           image,
		imageRepository: imageRepository,
		baseURL:         strings.TrimRight(baseURL, "/"),
		imageTTL:        imageTTL,
	}, nil
}

// RespondText handles POST /api/v1/chat/text
func (h *Handlers) RespondText(ctx context.Context, request apigen.RespondTextRequestObject) (apigen.RespondTextResponseObject, error) {
	if request.Body == nil {
		return apigen.RespondText400JSONResponse{Error: "bad_request"}, nil
	}

	response, err := h.text.SendMessage(request.Body.Text)
	if err != nil {
		return nil, err
	}

	// Map assistant messages to the public response shape.
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

// ChatImage handles POST /api/v1/chat/image (multipart/form-data)
func (h *Handlers) ChatImage(ctx context.Context, request apigen.ChatImageRequestObject) (apigen.ChatImageResponseObject, error) {
	if request.Body == nil {
		return apigen.ChatImage400JSONResponse{Error: "bad_request"}, nil
	}

	imageBytes, ctype, prompt, err := readSingleImagePart(request.Body)
	if err != nil {
		return apigen.ChatImage400JSONResponse{Error: err.Error()}, nil
	}
	if !isSupportedImage(ctype) {
		return apigen.ChatImage400JSONResponse{Error: "unsupported_media_type"}, nil
	}

	// Save image into temporary repo
	id, err := h.imageRepository.Save(ctx, imageBytes, h.imageTTL)
	if err != nil {
		return apigen.ChatImage500JSONResponse{Error: "internal_error"}, nil
	}

	imageURL := h.makeImageURL(id)

	// Ask the image model to read text from the image and respond
	response, err := h.image.SendImage(prompt, imageURL)
	if err != nil {
		return apigen.ChatImage500JSONResponse{Error: "model_error"}, nil
	}

	var items []apigen.ResponseItem
	for _, choice := range response.Choices {
		if choice.Message.Content == "" {
			continue
		}
		items = append(items, apigen.ResponseItem{
			Name:              response.Model,
			Description:       choice.Message.Content,
			MainImageUrl:      imageURL,
			CarouselImageUrls: []string{imageURL},
		})
	}
	if len(items) == 0 {
		items = []apigen.ResponseItem{{
			Name:              response.Model,
			Description:       "(empty response)",
			MainImageUrl:      imageURL,
			CarouselImageUrls: []string{imageURL},
		}}
	}

	return apigen.ChatImage200JSONResponse{Items: items}, nil
}

// GetStaticImage serves stored image bytes by UUID and deletes them after send.
func (h *Handlers) GetStaticImage(ctx context.Context, request apigen.GetStaticImageRequestObject) (apigen.GetStaticImageResponseObject, error) {
	id := request.Id.String()
	data, ok := h.imageRepository.Get(ctx, id)
	if !ok || len(data) == 0 {
		return apigen.GetStaticImage404JSONResponse{Error: "not_found"}, nil
	}

	ctype := http.DetectContentType(head(data))
	// Wrap the bytes with a reader that will delete (and optionally callback) on close.
	rdr := &deleteOnCloseReader{
		Reader: bytes.NewReader(data),
		onClose: func() {
			_ = h.imageRepository.Delete(context.Background(), id)
			// Optional callback
			if request.Params.Callback != nil && *request.Params.Callback != "" {
				go postCallback(*request.Params.Callback, id)
			}
		},
	}

	switch ctype {
	case "image/png":
		return apigen.GetStaticImage200ImagepngResponse{Body: rdr, ContentLength: int64(len(data))}, nil
	case "image/jpeg", "image/jpg":
		return apigen.GetStaticImage200ImagejpegResponse{Body: rdr, ContentLength: int64(len(data))}, nil
	default:
		// Default to jpeg content type if undetermined
		return apigen.GetStaticImage200ImagejpegResponse{Body: rdr, ContentLength: int64(len(data))}, nil
	}
}

// Helpers

func (h *Handlers) makeImageURL(id string) string {
	path := "/api/v1/images/" + id
	if h.baseURL == "" {
		return path
	}
	return h.baseURL + path
}

func isSupportedImage(ctype string) bool {
	switch ctype {
	case "image/png", "image/jpeg", "image/jpg":
		return true
	}
	return false
}

// readSingleImagePart reads a single part named fieldName from multipart.Reader.
func readSingleImagePart(r *multipart.Reader) ([]byte, string, string, error) {
	var prompt string
	var ctype string
	var buffer []byte

	for {
		part, err := r.NextPart()

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, "", "", err
		}
		defer part.Close()

		switch {
		case part.FormName() == "text":
			textBuffer, err := io.ReadAll(part)
			if err != nil {
				return nil, "", "", err
			}
			prompt = string(textBuffer)
			continue

		case part.FormName() == "image":
			buffer, err = io.ReadAll(part)
			if err != nil {
				return nil, "", "", err
			}
			ctype = http.DetectContentType(head(buffer))
			continue
		}

	}

	if len(buffer) != 0 && prompt != "" && ctype != "" {
		return buffer, ctype, prompt, nil
	}
	return nil, "", "", fmt.Errorf("failed to read form")
}

func head(b []byte) []byte {
	if len(b) > 512 {
		return b[:512]
	}
	return b
}

type deleteOnCloseReader struct {
	io.Reader
	onClose func()
}

func (r *deleteOnCloseReader) Close() error {
	if r.onClose != nil {
		r.onClose()
	}
	return nil
}

func postCallback(url string, id string) {
	payload := strings.NewReader(fmt.Sprintf(`{"id":"%s","status":"delivered"}`, id))
	req, err := http.NewRequest(http.MethodPost, url, payload)
	if err != nil {
		log.Printf("callback build error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// Use default client with a short timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("callback send error: %v", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("callback non-2xx status: %s", resp.Status)
	}
}
