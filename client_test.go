package jev

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvaluateRetriesOnRateLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"jev-1","answers":{"q":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":2}}`))
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL
	c.RetryBaseDelay = time.Millisecond

	resp, err := c.Evaluate(context.Background(), Request{Model: "jev-1", Questions: map[string]Question{
		"q": {Type: "noul", Instructions: "?"},
	}})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if resp.Answers["q"].Noul == nil || *resp.Answers["q"].Noul != 0.5 {
		t.Fatalf("resp.Answers[q] = %+v", resp.Answers["q"])
	}
}

func TestEvaluateNonRetryableError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClient("test-key")
	c.BaseURL = server.URL

	resp, err := c.Evaluate(context.Background(), Request{Model: "jev-1"})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want error")
	}
	if resp.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("resp.HTTPStatus = %d, want %d", resp.HTTPStatus, http.StatusUnauthorized)
	}
}
