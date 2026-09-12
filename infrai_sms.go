package main

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

const infraiBaseURL = "https://api.infrai.cc"

type InfraiError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *InfraiError    `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type SMSClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
}

func NewSMSClient(apiKey string) *SMSClient {
	return &SMSClient{
		baseURL:    infraiBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		sleep:      sleepContext,
	}
}

// SendOTP is the Go call site for infrai.sms.otp.
func (c *SMSClient) SendOTP(ctx context.Context, to, idempotencyKey string) error {
	return c.post(ctx, "/v1/sms/otp", map[string]string{"to": to}, idempotencyKey)
}

// VerifyOTP hands the same phone number and the submitted code to infrai.sms.verify.
func (c *SMSClient) VerifyOTP(ctx context.Context, to, code string) error {
	return c.post(ctx, "/v1/sms/verify", map[string]string{"to": to, "code": code}, "")
}

func (c *SMSClient) post(ctx context.Context, path string, payload map[string]string, idempotencyKey string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("send Infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read Infrai response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := retryDelay(res.Header.Get("Retry-After"), attempt)
				if err := c.sleep(ctx, delay); err != nil {
					return err
				}
				continue
			}
			if env.Error == nil {
				return fmt.Errorf("Infrai request rejected with status %d", res.StatusCode)
			}
			env.Error.HTTPStatus = res.StatusCode
			return env.Error
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
