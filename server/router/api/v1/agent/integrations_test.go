package agent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"testing"
)

func TestSignPayload(t *testing.T) {
	payload := []byte(`{"event":"test.ping","data":"hello"}`)
	secret := "test-secret-key"

	signature := signPayload(payload, secret)

	// Verify signature format
	if len(signature) == 0 {
		t.Fatal("signPayload returned empty signature")
	}
	if signature[:7] != "sha256=" {
		t.Fatalf("expected signature prefix 'sha256=', got %q", signature[:7])
	}

	// Verify signature is correct
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if signature != expected {
		t.Fatalf("signature mismatch:\n  got:  %q\n  want: %q", signature, expected)
	}

	// Verify different payloads produce different signatures
	differentPayload := []byte(`{"event":"different"}`)
	differentSignature := signPayload(differentPayload, secret)
	if signature == differentSignature {
		t.Fatal("different payloads should produce different signatures")
	}

	// Verify different secrets produce different signatures
	differentSecret := signPayload(payload, "other-secret")
	if signature == differentSecret {
		t.Fatal("different secrets should produce different signatures")
	}
}

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/webhook", false},
		{"valid http", "http://example.com/webhook", false},
		{"invalid scheme ftp", "ftp://example.com/webhook", true},
		{"invalid scheme file", "file:///etc/passwd", true},
		{"loopback", "http://127.0.0.1:8080/webhook", true},
		{"private ip", "http://192.168.1.1:8080/webhook", true},
		{"metadata ip", "http://169.254.169.254/latest/meta-data", true},
		{"empty host", "http:///webhook", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateAndResolveWebhookURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndResolveWebhookURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsInternalIP(t *testing.T) {
	tests := []struct {
		ip     string
		expect bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"169.254.169.254", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := isInternalIP(ip); got != tt.expect {
				t.Errorf("isInternalIP(%s) = %v, want %v", tt.ip, got, tt.expect)
			}
		})
	}
}

func TestComputeIdempotencyKey(t *testing.T) {
	key1 := computeIdempotencyKey("1", "lead-123", "lead.captured", "100")
	key2 := computeIdempotencyKey("1", "lead-123", "lead.captured", "100")
	key3 := computeIdempotencyKey("2", "lead-123", "lead.captured", "100")
	key4 := computeIdempotencyKey("1", "lead-456", "lead.captured", "100")
	key5 := computeIdempotencyKey("1", "lead-123", "lead.captured", "200")

	// Same inputs should produce same key
	if key1 != key2 {
		t.Fatalf("same inputs produced different keys: %q vs %q", key1, key2)
	}

	// Different tenant should produce different key
	if key1 == key3 {
		t.Fatal("different tenant IDs should produce different keys")
	}

	// Different lead should produce different key
	if key1 == key4 {
		t.Fatal("different lead IDs should produce different keys")
	}

	// Different integration should produce different key
	if key1 == key5 {
		t.Fatal("different integration IDs should produce different keys")
	}

	// Keys should be hex-encoded SHA256 (64 chars)
	if len(key1) != 64 {
		t.Fatalf("expected 64-char hex key, got %d chars", len(key1))
	}
}
