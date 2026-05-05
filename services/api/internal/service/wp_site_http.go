package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func newWPHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     newSafeWPTransport(),
		CheckRedirect: rejectWPRedirect,
	}
}

func (s *WpSiteService) Validate(ctx context.Context, baseURL string, username string, password string) error {
	normalizedBaseURL, err := NormalizeWPBaseURL(baseURL)
	if err != nil {
		return err
	}
	if err := s.checkRESTRoot(ctx, normalizedBaseURL); err != nil {
		return err
	}
	endpoint := strings.TrimRight(normalizedBaseURL, "/") + "/wp-json/wp/v2/users/me?context=edit"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("wp_request: %w", err)
	}
	req.SetBasicAuth(strings.TrimSpace(username), strings.TrimSpace(password))
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("wp_network: %w", err)
	}
	defer resp.Body.Close()
	if err := wpStatusError(resp.StatusCode); err != nil {
		return err
	}
	var payload struct {
		Capabilities map[string]bool `json:"capabilities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("wp_bad_response: %w", err)
	}
	if !payload.Capabilities["publish_posts"] {
		return ErrWpInsufficientCapability
	}
	return nil
}

func (s *WpSiteService) checkRESTRoot(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/wp-json/", nil)
	if err != nil {
		return fmt.Errorf("wp_root_request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("wp_root_network: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrWpRestAPINotFound
	}
	return wpStatusError(resp.StatusCode)
}

func newSafeWPTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, ErrWpPrivateAddress
			}
			for _, resolved := range ips {
				if !isPublicIP(resolved.IP) {
					return nil, ErrWpPrivateAddress
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

func rejectWPRedirect(_ *http.Request, _ []*http.Request) error {
	return ErrWpPrivateAddress
}

func isPublicIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
}
