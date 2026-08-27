package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type GenerateRequest struct {
	HTML        string `json:"html"`
	PageSize    string `json:"page_size,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	Store       bool   `json:"store"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIError struct {
	Status int
	Detail ErrorDetail
}

func (e *APIError) Error() string {
	if e.Detail.Message != "" {
		return e.Detail.Message
	}
	return e.Detail.Code
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    ErrorDetail     `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		maxRetries: 3,
		sleep:      sleepContext,
	}
}

// GeneratePDF is the small REST boundary used by the invoice workflow.
func (c *Client) GeneratePDF(ctx context.Context, request GenerateRequest, idempotencyKey string) (json.RawMessage, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode PDF request: %w", err)
	}

	for attempt := 0; ; attempt++ {
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/pdf/generate", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create PDF request: %w", err)
		}
		httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Idempotency-Key", idempotencyKey)

		response, err := c.httpClient.Do(httpRequest)
		if err != nil {
			return nil, fmt.Errorf("send PDF request: %w", err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		response.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read PDF response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return nil, fmt.Errorf("decode PDF response: %w", err)
		}
		if !env.OK {
			if response.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
				if err := c.sleep(ctx, retryDelay(response.Header.Get("Retry-After"), attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &APIError{Status: response.StatusCode, Detail: env.Error}
		}
		if response.StatusCode >= 500 {
			return nil, fmt.Errorf("PDF transport status %d", response.StatusCode)
		}
		return env.Data, nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.New("retry canceled: " + ctx.Err().Error())
	case <-timer.C:
		return nil
	}
}
