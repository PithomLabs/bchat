package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

var (
	// timeout is the timeout for webhook request. Default to 30 seconds.
	timeout = 30 * time.Second
)

// isInternalIP checks if an IP address is internal/private/link-local/metadata.
// H6: Used to block SSRF to internal services.
func isInternalIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// Cloud metadata IPs
	metadataIPs := []string{"169.254.169.254", "fd00:ec2::254"}
	for _, mIP := range metadataIPs {
		if ip.Equal(net.ParseIP(mIP)) {
			return true
		}
	}
	return false
}

// validateAndResolveWebhookURL validates the URL scheme and resolves DNS,
// rejecting internal IPs. Returns the validated IP for IP-pinned dialing.
// N4: Security enforcement at dispatch time (not just at save time).
func validateAndResolveWebhookURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.Wrap(err, "invalid webhook URL")
	}

	// Scheme allowlist
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.Errorf("only http/https schemes allowed, got: %s", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return "", errors.Errorf("missing hostname")
	}

	// Resolve DNS once
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve webhook host")
	}

	// Check ALL resolved IPs and pick the first valid external one
	var dialIP string
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isInternalIP(ip) {
			return "", errors.Errorf("webhook target resolves to internal IP: %s", addr)
		}
		if dialIP == "" {
			dialIP = addr
		}
	}

	if dialIP == "" {
		return "", errors.Errorf("no valid IP found for webhook host")
	}

	return dialIP, nil
}

// Post posts the message to webhook endpoint.
// N4: Uses IP-pinned dialer to prevent DNS-rebinding TOCTOU attacks.
func Post(requestPayload *v1pb.WebhookRequestPayload) error {
	body, err := protojson.Marshal(requestPayload)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal webhook request to %s", requestPayload.Url)
	}

	// N4: Validate URL and resolve DNS at dispatch time
	parsed, err := url.Parse(requestPayload.Url)
	if err != nil {
		return errors.Wrapf(err, "invalid webhook URL: %s", requestPayload.Url)
	}

	dialIP, err := validateAndResolveWebhookURL(requestPayload.Url)
	if err != nil {
		return errors.Wrapf(err, "webhook SSRF blocked for %s", requestPayload.Url)
	}

	// N4: IP-pinned transport — dial the validated IP, not re-resolved DNS
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Force connection to the validated IP
			_, port, _ := net.SplitHostPort(addr)
			if port == "" {
				port = "80"
				if parsed.Scheme == "https" {
					port = "443"
				}
			}
			targetAddr := net.JoinHostPort(dialIP, port)
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, targetAddr)
		},
	}

	// N4: Redirect policy — cap redirects + re-validate each target
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.Errorf("too many redirects")
			}
			// Re-validate redirect target
			_, err := validateAndResolveWebhookURL(req.URL.String())
			return err
		},
	}

	req, err := http.NewRequest("POST", requestPayload.Url, bytes.NewBuffer(body))
	if err != nil {
		return errors.Wrapf(err, "failed to construct webhook request to %s", requestPayload.Url)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to post webhook to %s", requestPayload.Url)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read webhook response from %s", requestPayload.Url)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.Errorf("failed to post webhook %s, status code: %d, response body: %s", requestPayload.Url, resp.StatusCode, b)
	}

	response := &struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{}
	if err := json.Unmarshal(b, response); err != nil {
		return errors.Wrapf(err, "failed to unmarshal webhook response from %s", requestPayload.Url)
	}

	if response.Code != 0 {
		return errors.Errorf("receive error code sent by webhook server, code %d, msg: %s", response.Code, response.Message)
	}

	return nil
}

// PostAsync posts the message to webhook endpoint asynchronously.
// It spawns a new goroutine to handle the request and does not wait for the response.
func PostAsync(requestPayload *v1pb.WebhookRequestPayload) {
	go func() {
		if err := Post(requestPayload); err != nil {
			// Since we're in a goroutine, we can only log the error
			slog.Warn("Failed to dispatch webhook asynchronously",
				slog.String("url", requestPayload.Url),
				slog.String("activityType", requestPayload.ActivityType),
				slog.Any("err", err))
		}
	}()
}
