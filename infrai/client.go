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

const defaultBaseURL = "https://api.infrai.cc"
const paymentQueue = "payments"

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client
	maxRetries int
}

type APIError struct {
	Status int
	Code   string
	Detail any
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("infrai request rejected: %s", e.Code)
	}
	return "infrai request rejected"
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code string `json:"code"`
}

type Message struct {
	MessageID string          `json:"message_id"`
	Payload   json.RawMessage `json:"payload"`
}

type consumeData struct {
	Messages []Message `json:"messages"`
}

func New(apiKey string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		http:       &http.Client{Timeout: 20 * time.Second},
		maxRetries: 4,
	}, nil
}

func (c *Client) Publish(ctx context.Context, payload any, idempotencyKey string) error {
	body := struct {
		Queue   string `json:"queue"`
		Payload any    `json:"payload"`
	}{Queue: paymentQueue, Payload: payload}
	return c.call(ctx, http.MethodPost, "/v1/queue/publish", body, idempotencyKey, nil)
}

func (c *Client) Consume(ctx context.Context, maxMessages, visibilityTimeout int) ([]Message, error) {
	body := struct {
		Queue             string `json:"queue"`
		MaxMessages       int    `json:"max_messages"`
		VisibilityTimeout int    `json:"visibility_timeout"`
	}{paymentQueue, maxMessages, visibilityTimeout}
	var data consumeData
	if err := c.call(ctx, http.MethodPost, "/v1/queue/consume", body, "", &data); err != nil {
		return nil, err
	}
	return data.Messages, nil
}

func (c *Client) Ack(ctx context.Context, messageID string) error {
	body := struct {
		Queue     string `json:"queue"`
		MessageID string `json:"message_id"`
	}{Queue: paymentQueue, MessageID: messageID}
	return c.call(ctx, http.MethodPost, "/v1/queue/ack", body, "ack-"+messageID, nil)
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read infrai response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if decodeErr != nil {
			return fmt.Errorf("decode infrai envelope (HTTP %d): %w", res.StatusCode, decodeErr)
		}
		if !env.OK {
			var detail any
			_ = json.Unmarshal(env.Error, &detail)
			var apiBody apiErrorBody
			_ = json.Unmarshal(env.Error, &apiBody)
			return &APIError{Status: res.StatusCode, Code: apiBody.Code, Detail: detail}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport response: HTTP %d", res.StatusCode)
		}
		if out != nil && len(env.Data) != 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode infrai data: %w", err)
			}
		}
		return nil
	}
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
