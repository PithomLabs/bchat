package agent

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/usememos/memos/internal/crypto"
	"github.com/usememos/memos/store"
)

var (
	keyIDRegex   = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
	numericRegex = regexp.MustCompile(`^[0-9]+$`)
)

func RequireBridgeHMAC(dbStore *store.Store, enc *crypto.EncryptionService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// 1. Query Parameters Rejection
			queryParams := c.QueryParams()
			for _, k := range []string{"key_id", "bridge_key", "bridge_client_id", "client_id", "api_key", "signature", "nonce"} {
				if queryParams.Has(k) {
					return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
				}
			}

			// 2. Nil Handling for EncryptionService
			if enc == nil {
				return echo.NewHTTPError(http.StatusInternalServerError, store.ErrBridgeAuthSecretUnavailable.Error())
			}

			// 3. Tenant Lookup & Status Check
			slug := c.Param("slug")
			if slug == "" {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}

			tenant, err := dbStore.GetAgentTenant(ctx, &store.FindAgentTenant{Slug: &slug})
			if err != nil {
				if errors.Is(err, store.ErrBridgeUnsupportedDatabase) {
					return echo.NewHTTPError(http.StatusNotImplemented, store.ErrBridgeUnsupportedDatabase.Error())
				}
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthKeyNotFound.Error())
			}
			if tenant == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthKeyNotFound.Error())
			}
			if !tenant.IsActive {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInactiveTenant.Error())
			}

			// 4. Content-Type Validation and Canonicalization
			contentTypeHeader := c.Request().Header.Get("Content-Type")
			if contentTypeHeader == "" {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}
			mediaType, params, err := mime.ParseMediaType(contentTypeHeader)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}
			if mediaType != "application/json" {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}
			if charset, ok := params["charset"]; ok {
				if strings.ToLower(charset) != "utf-8" {
					return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
				}
			}

			// 5. Header & Format Validation
			authHeader := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				// Case sensitive Bearer check: rejects lowercase bearer or anything else
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidSignature.Error())
			}
			keyID := strings.TrimPrefix(authHeader, "Bearer ")
			if !keyIDRegex.MatchString(keyID) {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}

			tsStr := c.Request().Header.Get("X-Bridge-Timestamp")
			if tsStr == "" {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}
			if !numericRegex.MatchString(tsStr) {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidTimestamp.Error())
			}
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidTimestamp.Error())
			}

			// Freshness validation (±5 minutes)
			now := time.Now()
			diff := now.Unix() - ts
			if diff < -300 || diff > 300 {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidTimestamp.Error())
			}

			nonce := c.Request().Header.Get("X-Bridge-Nonce")
			if nonce == "" {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}
			if !keyIDRegex.MatchString(nonce) {
				return echo.NewHTTPError(http.StatusBadRequest, store.ErrBridgeAuthMalformedRequest.Error())
			}

			sigStr := c.Request().Header.Get("X-Bridge-Signature")
			if sigStr == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidSignature.Error())
			}
			if !strings.HasPrefix(sigStr, "v1=") {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidSignature.Error())
			}
			providedSigStr := strings.TrimPrefix(sigStr, "v1=")
			if providedSigStr == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidSignature.Error())
			}
			providedBytes, err := base64.RawURLEncoding.DecodeString(providedSigStr)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidSignature.Error())
			}
			if len(providedBytes) != 32 {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidSignature.Error())
			}

			// 6. Body Size Limit (1 MiB max)
			const maxBodyBytes = 1 * 1024 * 1024 // 1 MiB
			limitedReader := io.LimitReader(c.Request().Body, maxBodyBytes + 1)
			bodyBytes, err := io.ReadAll(limitedReader)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read request body")
			}
			if len(bodyBytes) > maxBodyBytes {
				return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "Request body exceeds 1 MiB limit")
			}
			c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// 7. Client Lookup
			authKey, err := dbStore.GetBridgeAuthKey(ctx, tenant.ID, keyID)
			if err != nil {
				if errors.Is(err, store.ErrBridgeAuthKeyNotFound) {
					return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthKeyNotFound.Error())
				}
				if errors.Is(err, store.ErrBridgeUnsupportedDatabase) {
					return echo.NewHTTPError(http.StatusNotImplemented, store.ErrBridgeUnsupportedDatabase.Error())
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to look up auth key")
			}
			if authKey.Status != "active" {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthKeyRevoked.Error())
			}

			// Decrypt secret
			rawSecret, err := enc.Decrypt(authKey.SecretKeyEncrypted, authKey.SecretKeyNonce)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, store.ErrBridgeAuthSecretUnavailable.Error())
			}

			// 8. Canonicalization
			bodyHash := sha256.Sum256(bodyBytes)
			bodyHashBase64 := base64.RawURLEncoding.EncodeToString(bodyHash[:])
			method := strings.ToUpper(c.Request().Method)
			path := c.Request().URL.EscapedPath()

			canonicalString := fmt.Sprintf(
				"BCHAT-BRIDGE-V1\n%s\n%s\n%s\n%s\n%s\n%d\n%s\n%s",
				keyID,
				tenant.Slug,
				method,
				path,
				"application/json", // exactly application/json
				ts,
				nonce,
				bodyHashBase64,
			)

			// 9. Signature Verification
			mac := hmac.New(sha256.New, []byte(rawSecret))
			mac.Write([]byte(canonicalString))
			expectedMAC := mac.Sum(nil)

			if !hmac.Equal(providedBytes, expectedMAC) {
				return echo.NewHTTPError(http.StatusUnauthorized, store.ErrBridgeAuthInvalidSignature.Error())
			}

			// 10. Nonce Replay Protection (Only after successful signature verification)
			nonceRecord := &store.BridgeAuthNonce{
				TenantID:  tenant.ID,
				KeyID:     keyID,
				Nonce:     nonce,
				Timestamp: ts,
				CreatedAt: now,
				ExpiresAt: now.Add(15 * time.Minute),
			}
			err = dbStore.StoreBridgeAuthNonce(ctx, nonceRecord)
			if err != nil {
				if errors.Is(err, store.ErrBridgeAuthReplay) {
					return echo.NewHTTPError(http.StatusConflict, store.ErrBridgeAuthReplay.Error())
				}
				if errors.Is(err, store.ErrBridgeUnsupportedDatabase) {
					return echo.NewHTTPError(http.StatusNotImplemented, store.ErrBridgeUnsupportedDatabase.Error())
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to store nonce")
			}

			// 11. Update Last Used (Log error but do not fail the request)
			err = dbStore.UpdateBridgeAuthKeyLastUsed(ctx, tenant.ID, keyID, now)
			if err != nil {
				slog.Error("failed to update bridge auth key last used time", "error", err, "tenant_id", tenant.ID, "key_id", keyID)
			}

			return next(c)
		}
	}
}
