package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientDecodesBusinessErrorBeforeStatus(t *testing.T) {
	client := NewSMSClient("test-key")
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sms/verify" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusUnprocessableEntity,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":false,"data":null,"error":{"code":"CODE_REJECTED","message":"code rejected"},"metadata":{}}`)),
		}, nil
	})

	err := client.VerifyOTP(context.Background(), "+15551234567", "000000")
	apiErr, ok := err.(*InfraiError)
	if !ok || apiErr.Code != "CODE_REJECTED" || apiErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("error = %#v", err)
	}
}
