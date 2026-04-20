package helpers

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestDefaultHTTPClientConfig_Values(t *testing.T) {
	cfg := DefaultHTTPClientConfig()
	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Timeout", cfg.Timeout, 30 * time.Second},
		{"DialTimeout", cfg.DialTimeout, 10 * time.Second},
		{"TLSHandshakeTimeout", cfg.TLSHandshakeTimeout, 10 * time.Second},
		{"MaxIdleConns", cfg.MaxIdleConns, 10},
		{"IdleConnTimeout", cfg.IdleConnTimeout, 90 * time.Second},
		{"DisableKeepAlives", cfg.DisableKeepAlives, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestNewHTTPClient_EmptyProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"invalid scheme", "://bad", true},
		{"missing host", "socks5://", false},
		{"valid socks5", "socks5://127.0.0.1:9050", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSimpleClient{proxyURL: tt.proxyURL}
			_, err := NewHTTPClient(client, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewHTTPTransport_EmptyProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{"empty string", "", true},
		{"invalid scheme", "://bad", true},
		{"valid socks5", "socks5://127.0.0.1:9050", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSimpleClient{proxyURL: tt.proxyURL}
			_, err := NewHTTPTransport(client, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWrapHTTPClient_NilHTTPClient(t *testing.T) {
	tests := []struct {
		name       string
		httpClient *http.Client
		torClient  TorClient
		wantErr    bool
	}{
		{"nil http client", nil, &mockSimpleClient{proxyURL: "socks5://127.0.0.1:9050"}, true},
		{"nil tor client", &http.Client{}, nil, true},
		{"both nil", nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapHTTPClient(tt.httpClient, tt.torClient, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDialContext_CancelledContext(t *testing.T) {
	client := &mockSimpleClient{proxyURL: "socks5://127.0.0.1:9050"}
	dialFn := DialContext(client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := dialFn(ctx, "tcp", "example.com:80")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestNewHTTPClient_ZeroValueConfig(t *testing.T) {
	client := &mockSimpleClient{proxyURL: "socks5://127.0.0.1:9050"}
	cfg := &HTTPClientConfig{} // all zero values
	httpClient, err := NewHTTPClient(client, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpClient == nil {
		t.Fatal("expected non-nil http client")
	}
	if httpClient.Timeout != 0 {
		t.Errorf("expected zero timeout, got %v", httpClient.Timeout)
	}
}
