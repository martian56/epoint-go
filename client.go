package epoint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const productionURL = "https://epoint.az"

type Client struct {
	publicKey          string
	privateKey         string
	baseURL            string
	language           string
	currency           string
	successRedirectURL string
	errorRedirectURL   string
	http               *http.Client
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

func WithLanguage(language string) Option {
	return func(c *Client) { c.language = language }
}

func WithCurrency(currency string) Option {
	return func(c *Client) { c.currency = currency }
}

func WithRedirectURLs(success, failure string) Option {
	return func(c *Client) {
		c.successRedirectURL = success
		c.errorRedirectURL = failure
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.http = httpClient }
}

func New(publicKey, privateKey string, opts ...Option) *Client {
	c := &Client{
		publicKey:  publicKey,
		privateKey: privateKey,
		baseURL:    productionURL,
		language:   string(LanguageAZ),
		currency:   string(CurrencyAZN),
		http:       &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func FromEnv(opts ...Option) (*Client, error) {
	publicKey := os.Getenv("EPOINT_PUBLIC_KEY")
	privateKey := os.Getenv("EPOINT_PRIVATE_KEY")
	if publicKey == "" || privateKey == "" {
		return nil, fmt.Errorf("EPOINT_PUBLIC_KEY and EPOINT_PRIVATE_KEY must be set")
	}
	base := []Option{
		WithLanguage(getenv("EPOINT_LANGUAGE", "az")),
		WithRedirectURLs(os.Getenv("EPOINT_SUCCESS_REDIRECT_URL"), os.Getenv("EPOINT_FAILED_REDIRECT_URL")),
	}
	if u := os.Getenv("EPOINT_BASE_URL"); u != "" {
		base = append(base, WithBaseURL(u))
	}
	return New(publicKey, privateKey, append(base, opts...)...), nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func money(amount float64) string {
	return strconv.FormatFloat(amount, 'f', 2, 64)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (c *Client) send(ctx context.Context, method, path string, body io.Reader, contentType string) (int, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, &TransportError{Message: err.Error()}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, &TransportError{Message: err.Error()}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, &TransportError{Message: err.Error(), StatusCode: resp.StatusCode}
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		snippet := string(raw)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return resp.StatusCode, nil, &TransportError{
			Message:    "response was not a JSON object: " + snippet,
			StatusCode: resp.StatusCode,
		}
	}
	return resp.StatusCode, parsed, nil
}

func (c *Client) sendForm(ctx context.Context, method, path string, payload map[string]any) (int, map[string]any, error) {
	body, err := c.buildBody(payload)
	if err != nil {
		return 0, nil, err
	}
	form := url.Values{"data": {body.data}, "signature": {body.signature}}
	return c.send(ctx, method, path, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

func (c *Client) post(ctx context.Context, path string, payload map[string]any) (*Response, error) {
	status, parsed, err := c.sendForm(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	return parse(status, parsed)
}

func (c *Client) postRaw(ctx context.Context, path string, payload map[string]any) (any, error) {
	body, err := c.buildBody(payload)
	if err != nil {
		return nil, err
	}
	form := url.Values{"data": {body.data}, "signature": {body.signature}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &TransportError{Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &TransportError{Message: err.Error()}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &TransportError{Message: err.Error(), StatusCode: resp.StatusCode}
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, &TransportError{Message: "response was not JSON", StatusCode: resp.StatusCode}
	}
	if obj, ok := value.(map[string]any); ok {
		if _, err := parse(resp.StatusCode, obj); err != nil {
			return nil, err
		}
	}
	return value, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload map[string]any) (*Response, error) {
	body, err := c.buildBody(payload)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(map[string]string{"data": body.data, "signature": body.signature})
	if err != nil {
		return nil, &TransportError{Message: err.Error()}
	}
	status, parsed, err := c.send(ctx, http.MethodPost, path, strings.NewReader(string(encoded)), "application/json")
	if err != nil {
		return nil, err
	}
	return parse(status, parsed)
}

type signedBody struct {
	data      string
	signature string
}

func (c *Client) buildBody(payload map[string]any) (signedBody, error) {
	full := map[string]any{"public_key": c.publicKey}
	for key, value := range payload {
		if value == nil {
			continue
		}
		if s, ok := value.(string); ok && s == "" {
			continue
		}
		full[key] = value
	}
	data, err := encodePayload(full)
	if err != nil {
		return signedBody{}, err
	}
	return signedBody{data: data, signature: sign(c.privateKey, data)}, nil
}

func parse(statusCode int, body map[string]any) (*Response, error) {
	if body["status"] == string(StatusError) {
		return nil, &GatewayError{
			Message: stringOr(body["message"], "request refused"),
			Code:    stringOr(body["code"], ""),
			Status:  stringOr(body["status"], ""),
			Payload: body,
		}
	}
	if statusCode >= 400 {
		detail := stringOr(body["detail"], stringOr(body["message"], fmt.Sprintf("HTTP %d", statusCode)))
		return nil, &TransportError{Message: detail, StatusCode: statusCode}
	}
	return &Response{Raw: body}, nil
}

func stringOr(value any, fallback string) string {
	switch v := value.(type) {
	case nil:
		return fallback
	case string:
		if v == "" {
			return fallback
		}
		return v
	default:
		return fmt.Sprint(v)
	}
}
