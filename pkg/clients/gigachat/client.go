package gigachat

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	apigen "pod_api/pkg/apigen/gigachat"
	"pod_api/pkg/config"
	"pod_api/pkg/models"

	"github.com/google/uuid"
)

const (
	systemPromt    = "system_stub_prompt"
	assistantPromt = "assistant_stub_prompt"
	functionPromt  = "function_stub_prompt"
)

type Client struct {
	baseURL string

	// Basic credentials for token fetching (base64-encoded client:secret)
	basicKey string

	// OAuth scope and default model
	scope     apigen.PostTokenFormdataBodyScope
	model     string
	maxTokens int32

	// Generated API clients
	apiClient   *apigen.ClientWithResponses
	tokenClient *apigen.ClientWithResponses
	httpClient  *http.Client

	// Bearer token management
	tokenMu       sync.RWMutex
	accessToken   string
	tokenExpiry   time.Time
	refreshLeeway time.Duration
	stopCh        chan struct{}
}

// NewClient constructs a GigaChat client.
// url — API base URL (e.g. https://gigachat.devices.sberbank.ru/api/v1)
// key — base64-encoded Basic credentials (without the "Basic " prefix)
//
// Defaults: scope=GIGACHAT_API_PERS, model=GigaChat, refreshLeeway=10s.
func NewClient(url string, key string) (*Client, error) {
	return NewClientWithOptions(url, key, NewOptions())
}

// Options controls optional parameters for NewClientWithOptions.
type Options struct {
	Scope         apigen.PostTokenFormdataBodyScope
	Model         string
	RefreshLeeway time.Duration
	MaxTokens     int32
}

// NewOptions returns sensible defaults.
func NewOptions() Options {
	return Options{
		Scope:         apigen.GIGACHATAPIPERS,
		Model:         "GigaChat-2",
		RefreshLeeway: 10 * time.Second,
		MaxTokens:     1024,
	}
}

// NewClientWithOptions allows specifying scope/model/refresh leeway.
func NewClientWithOptions(url string, key string, opts Options) (*Client, error) {
	if url == "" {
		return nil, errors.New("empty base URL")
	}
	if key == "" {
		return nil, errors.New("empty basic key")
	}

	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 1024
	}

	c := &Client{
		baseURL:       url,
		basicKey:      key,
		scope:         opts.Scope,
		model:         opts.Model,
		refreshLeeway: opts.RefreshLeeway,
		stopCh:        make(chan struct{}),
		maxTokens:     opts.MaxTokens,
	}

	// API client for chat and other methods; attach bearer editor
	apiClient, err := apigen.NewClientWithResponses(url, apigen.WithRequestEditorFn(c.bearerAuthEditor))
	if err != nil {
		return nil, err
	}
	c.apiClient = apiClient

	// Token client (no default editors; we pass Basic per request)
	tokenClient, err := apigen.NewClientWithResponses(url)
	if err != nil {
		return nil, err
	}
	c.tokenClient = tokenClient

	// Fetch token immediately
	if err := c.refreshToken(context.Background()); err != nil {
		return nil, err
	}

	// Start background refresh
	go c.tokenRefresher()

	return c, nil
}

// NewFromConfig constructs a client from app config.
// Expects cfg.Gigachat.BasicKey to be set (base64 client:secret).
func NewFromConfig(cfg config.Config) (*Client, error) {
	opts := NewOptions()
	if cfg.Gigachat.Model != "" {
		opts.Model = cfg.Gigachat.Model
	}
	if cfg.Gigachat.TokenRefreshLeewaySeconds > 0 {
		opts.RefreshLeeway = time.Duration(cfg.Gigachat.TokenRefreshLeewaySeconds) * time.Second
	}
	if cfg.Gigachat.Scope != "" {
		opts.Scope = apigen.PostTokenFormdataBodyScope(cfg.Gigachat.Scope)
	}
	if cfg.Gigachat.MaxTokens > 0 {
		opts.MaxTokens = int32(cfg.Gigachat.MaxTokens)
	}
	if cfg.Gigachat.BasicKey == "" {
		return nil, errors.New("GIGACHAT_BASIC_KEY is empty")
	}
	// Build HTTP client with custom Root CA fetched from URL
	httpClient, err := buildHTTPClientWithRootCA(cfg.Gigachat.RootCAURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		baseURL:       cfg.Gigachat.URL,
		basicKey:      cfg.Gigachat.BasicKey,
		scope:         opts.Scope,
		model:         opts.Model,
		refreshLeeway: opts.RefreshLeeway,
		stopCh:        make(chan struct{}),
		httpClient:    httpClient,
		maxTokens:     opts.MaxTokens,
	}

	// API + token clients using the custom HTTP client
	apiClient, err := apigen.NewClientWithResponses(cfg.Gigachat.URL,
		apigen.WithHTTPClient(httpClient),
		apigen.WithRequestEditorFn(c.bearerAuthEditor),
	)
	if err != nil {
		return nil, err
	}

	tokenClient, err := apigen.NewClientWithResponses(cfg.Gigachat.AuthURL, apigen.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	c.apiClient = apiClient
	c.tokenClient = tokenClient

	if err := c.refreshToken(context.Background()); err != nil {
		return nil, err
	}
	go c.tokenRefresher()
	return c, nil
}

// buildHTTPClientWithRootCA downloads a PEM certificate from URL and returns an HTTP client that trusts it.
func buildHTTPClientWithRootCA(certURL string) (*http.Client, error) {
	if certURL == "" {
		return nil, errors.New("empty Root CA URL")
	}

	insecureTr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	insecureCli := &http.Client{Transport: insecureTr, Timeout: 30 * time.Second}
	resp, err := insecureCli.Get(certURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	pemBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Optional validate first block exists; AppendCertsFromPEM will still parse multiple blocks
	if block, _ := pem.Decode(pemBytes); block == nil {
		// proceed anyway; AppendCertsFromPEM will validate
	}

	pool, _ := x509.SystemCertPool()
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if ok := pool.AppendCertsFromPEM(pemBytes); !ok {
		return nil, errors.New("failed to append Root CA PEM")
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
	return &http.Client{Transport: tr, Timeout: 60 * time.Second}, nil
}

// bearerAuthEditor injects the current bearer token.
func (c *Client) bearerAuthEditor(_ context.Context, req *http.Request) error {
	if token := c.getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

// basicAuthEditor injects Basic auth for token fetching.
func (c *Client) basicAuthEditor(_ context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Basic "+c.basicKey)
	req.Header.Set("Accept", "application/json")
	return nil
}

// getToken returns the current access token in a thread-safe manner.
func (c *Client) getToken() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.accessToken
}

// setToken sets the access token and expiry.
func (c *Client) setToken(token string, exp time.Time) {
	c.tokenMu.Lock()
	c.accessToken = token
	c.tokenExpiry = exp
	c.tokenMu.Unlock()
}

// nextRefreshDelay computes when to refresh the token.
func (c *Client) nextRefreshDelay() time.Duration {
	c.tokenMu.RLock()
	exp := c.tokenExpiry
	c.tokenMu.RUnlock()
	d := max(time.Until(exp)-c.refreshLeeway, time.Second)
	return d
}

// refreshToken obtains a new token via /oauth.
func (c *Client) refreshToken(ctx context.Context) error {
	// Prepare request params
	params := &apigen.PostTokenParams{RqUID: uuid.NewString()}
	body := apigen.PostTokenFormdataRequestBody{
		Scope: c.scope,
	}

	response, err := c.tokenClient.PostTokenWithFormdataBodyWithResponse(ctx, params, body, c.basicAuthEditor)
	if err != nil {
		return err
	}

	statusCode := response.StatusCode()
	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s, request: %v", statusCode, response.Body, response.HTTPResponse.Request)
	}
	if response.JSON200 == nil || response.JSON200.AccessToken == nil || response.JSON200.ExpiresAt == nil {
		return fmt.Errorf("unexpected token status: %d", response.StatusCode())
	}

	token := *response.JSON200.AccessToken
	exp := time.UnixMilli(int64(*response.JSON200.ExpiresAt))
	c.setToken(token, exp)
	return nil
}

// tokenRefresher refreshes token ~leeway before expiry, with simple retry.
func (c *Client) tokenRefresher() {
	for {
		// Wait until time to refresh or stop
		delay := c.nextRefreshDelay()
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			// try to refresh; on error, retry after 5s
			if err := c.refreshToken(context.Background()); err != nil {
				time.Sleep(5 * time.Second)
				_ = c.refreshToken(context.Background())
			}
		case <-c.stopCh:
			timer.Stop()
			return
		}
	}
}

// Close stops background token refresh.
func (c *Client) Close() {
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
}

func makePromt(userMessage string) []apigen.Message {
	// TODO: Implement proper prompts

	// sysRole := apigen.MessageRoleSystem
	// sysContent := systemPromt

	// assistantRole := apigen.MessageRoleAssistant
	// assistantContent := assistantPromt

	// functionRole := apigen.MessageRoleFunction
	// functionContent := functionPromt

	userRole := apigen.MessageRoleUser
	userContent := userMessage

	return []apigen.Message{
		// {Role: &sysRole, Content: &sysContent},
		{Role: &userRole, Content: &userContent},
		// {Role: &assistantRole, Content: &assistantContent},
		// {Role: &functionRole, Content: &functionContent},
	}
}

// SendMessage implements api.TextModel: sends a message to chat completions.
func (c *Client) SendMessage(userMessage string) (*models.ChatResponse, error) {
	if userMessage == "" {
		return nil, errors.New("empty message")
	}

	messages := makePromt(userMessage)

	maxTokens := c.maxTokens
	request := apigen.Chat{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: &maxTokens,
	}

	// Execute request with bearer editor (already attached globally).
	response, err := c.apiClient.PostChatWithResponse(context.Background(), nil, request)
	if err != nil {
		return nil, err
	}

	// Accept 200 (JSON or stream) as success. Any error JSON codes -> error.
	if response.JSON200 == nil && (response.StatusCode() != 200) {
		return nil, errors.New("chat request failed: status " + response.Status())
	}

	// Map GigaChat response to unified ChatResponse
	if response.JSON200 == nil {
		// No JSON body (maybe streaming or empty)
		return &models.ChatResponse{}, nil
	}

	gc := response.JSON200
	out := &models.ChatResponse{}
	if gc.Created != nil {
		out.Created = int64(*gc.Created)
	}
	if gc.Model != nil {
		out.Model = *gc.Model
	}
	if gc.Object != nil {
		out.Object = *gc.Object
	}

	// Map usage
	if gc.Usage != nil {
		u := &models.ChatUsage{}
		if gc.Usage.PromptTokens != nil {
			u.PromptTokens = *gc.Usage.PromptTokens
		}
		if gc.Usage.CompletionTokens != nil {
			u.CompletionTokens = *gc.Usage.CompletionTokens
		}
		if gc.Usage.TotalTokens != nil {
			u.TotalTokens = *gc.Usage.TotalTokens
		}
		if gc.Usage.PrecachedPromptTokens != nil {
			u.PrecachedPromptTokens = *gc.Usage.PrecachedPromptTokens
		}
		out.Usage = u
	}

	// Map choices/messages
	if gc.Choices != nil {
		for _, ch := range *gc.Choices {
			choice := models.ChatChoice{}
			if ch.Index != nil {
				choice.Index = *ch.Index
			}
			if ch.FinishReason != nil {
				choice.FinishReason = string(*ch.FinishReason)
			}
			if ch.Message != nil {
				m := models.ChatMessage{}
				if ch.Message.Content != nil {
					m.Content = *ch.Message.Content
				}
				if ch.Message.Role != nil {
					m.Role = string(*ch.Message.Role)
				}
				if ch.Message.Name != nil {
					m.Name = *ch.Message.Name
				}
				if ch.Message.Created != nil {
					m.Created = int64(*ch.Message.Created)
				}
				if ch.Message.FunctionsStateId != nil {
					m.FunctionsStateID = *ch.Message.FunctionsStateId
				}
				if ch.Message.FunctionCall != nil {
					fc := &models.FunctionCall{}
					if ch.Message.FunctionCall.Name != nil {
						fc.Name = *ch.Message.FunctionCall.Name
					}
					if ch.Message.FunctionCall.Arguments != nil {
						fc.Arguments = *ch.Message.FunctionCall.Arguments
					}
					m.FunctionCall = fc
				}
				choice.Message = m
			}
			out.Choices = append(out.Choices, choice)
		}
	}

	return out, nil
}
