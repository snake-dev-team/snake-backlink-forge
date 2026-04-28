package service

import (
	"errors"
	"net/http"
	"testing"
)

func TestNormalizeWPBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https", raw: " HTTPS://Example.COM/blog/?a=1#frag ", want: "https://example.com/blog"},
		{name: "localhost http", raw: "http://localhost:8080/wp/", want: "http://localhost:8080/wp"},
		{name: "ipv4 local http", raw: "http://127.0.0.1:8080/", want: "http://127.0.0.1:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeWPBaseURL(tt.raw)
			if err != nil {
				t.Fatalf("NormalizeWPBaseURL: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWPBaseURLRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "example.com", "http://example.com", "ftp://example.com"} {
		t.Run(raw, func(t *testing.T) {
			_, err := NormalizeWPBaseURL(raw)
			if !errors.Is(err, ErrWpInvalidURL) {
				t.Fatalf("err = %v, want ErrWpInvalidURL", err)
			}
		})
	}
}

func TestWpStatusError(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{status: http.StatusOK, want: nil},
		{status: http.StatusUnauthorized, want: ErrWpInvalidCredentials},
		{status: http.StatusForbidden, want: ErrWpInsufficientCapability},
		{status: http.StatusNotFound, want: ErrWpRestAPINotFound},
		{status: http.StatusTooManyRequests, want: ErrWpRateLimited},
		{status: http.StatusBadGateway, want: ErrWpServerError502},
		{status: http.StatusServiceUnavailable, want: ErrWpServerError503},
		{status: http.StatusGatewayTimeout, want: ErrWpServerError504},
	}

	for _, tt := range tests {
		err := wpStatusError(tt.status)
		if !errors.Is(err, tt.want) {
			t.Fatalf("status %d err = %v, want %v", tt.status, err, tt.want)
		}
	}
}

func TestWpErrorCode(t *testing.T) {
	if got := WpErrorCode(ErrWpInvalidCredentials); got != "wp_invalid_credentials" {
		t.Fatalf("got %q", got)
	}
	if got := WpErrorCode(errors.New("other")); got != "wp_network" {
		t.Fatalf("got %q", got)
	}
}

func TestSafeWPTransportRejectsPrivateAddress(t *testing.T) {
	conn, err := newSafeWPTransport().DialContext(t.Context(), "tcp", "10.0.0.1:80")
	if conn != nil {
		conn.Close()
	}
	if !errors.Is(err, ErrWpPrivateAddress) {
		t.Fatalf("err = %v, want ErrWpPrivateAddress", err)
	}
}
