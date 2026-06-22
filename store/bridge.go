package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

type BridgeExternalSessionStatus string

const (
	BridgeExternalSessionStatusActive  BridgeExternalSessionStatus = "active"
	BridgeExternalSessionStatusClosed  BridgeExternalSessionStatus = "closed"
	BridgeExternalSessionStatusExpired BridgeExternalSessionStatus = "expired"
)

type BridgeRoutingMode string

const (
	BridgeRoutingModeHandoffQueued BridgeRoutingMode = "handoff_queued"
	BridgeRoutingModeHumanActive   BridgeRoutingMode = "human_active"
	BridgeRoutingModeClosed        BridgeRoutingMode = "closed"
)

type BridgeOutcome string

const (
	BridgeOutcomeReleased        BridgeOutcome = "released"
	BridgeOutcomeTimeoutReleased BridgeOutcome = "timeout_released"
	BridgeOutcomeResolved        BridgeOutcome = "resolved"
	BridgeOutcomeRejected        BridgeOutcome = "rejected"
	BridgeOutcomeFailed          BridgeOutcome = "failed"
	BridgeOutcomeClosed          BridgeOutcome = "closed"
)

var (
	ErrInvalidExternalSessionID      = errors.New("invalid external session id")
	ErrBridgeExternalSessionNotFound = errors.New("bridge external session not found")
	ErrBridgeHandoffNotFound         = errors.New("bridge handoff not found")
	ErrBridgeHandoffConflict         = errors.New("bridge handoff conflict")
	ErrBridgeUnsupportedDatabase     = errors.New("bridge runtime unsupported for database")

	externalSessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type BridgeExternalSession struct {
	ID         int64
	TenantID   int32
	SessionID  string
	Status     BridgeExternalSessionStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  *time.Time
	LastSeenAt *time.Time
}

type BridgeHandoff struct {
	ID                int64
	ExternalSessionID int64
	HandoffID         string
	TenantID          int32
	SessionID         string
	Generation        int
	RoutingMode       BridgeRoutingMode
	Outcome           *BridgeOutcome
	Active            bool
	Version           int
	HarnessID         *string
	OperatorID        *string
	TicketID          *int32
	MemoUID           *string
	TransitionReason  *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ClosedAt          *time.Time
}

func ValidateExternalSessionID(sessionID string) error {
	if !externalSessionIDPattern.MatchString(sessionID) {
		return fmt.Errorf("%w: must be 1-128 ASCII letters, digits, underscores, or hyphens", ErrInvalidExternalSessionID)
	}
	return nil
}

func ValidateBridgeRoutingMode(mode BridgeRoutingMode) error {
	switch mode {
	case BridgeRoutingModeHandoffQueued, BridgeRoutingModeHumanActive, BridgeRoutingModeClosed:
		return nil
	default:
		return fmt.Errorf("invalid bridge routing mode %q", mode)
	}
}

func ValidateBridgeOutcome(outcome BridgeOutcome) error {
	switch outcome {
	case BridgeOutcomeReleased, BridgeOutcomeTimeoutReleased, BridgeOutcomeResolved,
		BridgeOutcomeRejected, BridgeOutcomeFailed, BridgeOutcomeClosed:
		return nil
	default:
		return fmt.Errorf("invalid bridge outcome %q", outcome)
	}
}

func (s *Store) EnsureBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) (*BridgeExternalSession, bool, error) {
	return s.driver.EnsureBridgeExternalSession(ctx, tenantID, sessionID, now, expiresAt)
}

func (s *Store) FindBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string) (*BridgeExternalSession, error) {
	return s.driver.FindBridgeExternalSession(ctx, tenantID, sessionID)
}

func (s *Store) TouchBridgeExternalSession(ctx context.Context, tenantID int32, sessionID string, now, expiresAt time.Time) error {
	return s.driver.TouchBridgeExternalSession(ctx, tenantID, sessionID, now, expiresAt)
}

func (s *Store) CreateBridgeHandoff(ctx context.Context, tenantID int32, sessionID string, now time.Time) (*BridgeHandoff, error) {
	return s.driver.CreateBridgeHandoff(ctx, tenantID, sessionID, now)
}

func (s *Store) FindActiveBridgeHandoff(ctx context.Context, tenantID int32, sessionID string) (*BridgeHandoff, error) {
	return s.driver.FindActiveBridgeHandoff(ctx, tenantID, sessionID)
}

func (s *Store) UpdateBridgeHandoffRoutingModeCAS(ctx context.Context, tenantID int32, sessionID string, generation int, handoffID string, fromVersion int, fromMode, toMode BridgeRoutingMode, reason string, now time.Time) (*BridgeHandoff, error) {
	return s.driver.UpdateBridgeHandoffRoutingModeCAS(ctx, tenantID, sessionID, generation, handoffID, fromVersion, fromMode, toMode, reason, now)
}

func (s *Store) GetBridgeHandoff(ctx context.Context, tenantID int32, sessionID string, handoffID string) (*BridgeHandoff, error) {
	return s.driver.GetBridgeHandoff(ctx, tenantID, sessionID, handoffID)
}

var (
	ErrBridgeAuthReplay           = errors.New("bridge auth replay detected")
	ErrBridgeAuthKeyNotFound       = errors.New("bridge auth key not found")
	ErrBridgeAuthInactiveTenant   = errors.New("bridge auth tenant is inactive")
	ErrBridgeAuthInvalidTimestamp = errors.New("bridge auth invalid timestamp")
	ErrBridgeAuthSecretUnavailable = errors.New("bridge auth encryption service unavailable")
	ErrBridgeAuthMalformedRequest = errors.New("bridge auth malformed request")
	ErrBridgeAuthInvalidSignature = errors.New("bridge auth invalid signature")
	ErrBridgeAuthKeyRevoked       = errors.New("bridge auth key is revoked")
)

type BridgeAuthKey struct {
	ID                 int64      `json:"id"`
	TenantID           int32      `json:"tenant_id"`
	KeyID              string     `json:"key_id"`
	Label              *string    `json:"label"`
	SecretKeyEncrypted []byte     `json:"-"`
	SecretKeyNonce     []byte     `json:"-"`
	Status             string     `json:"status"` // "active" or "revoked"
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastUsedAt         *time.Time `json:"last_used_at"`
	RevokedAt          *time.Time `json:"revoked_at"`
}

type BridgeAuthNonce struct {
	ID        int64     `json:"id"`
	TenantID  int32     `json:"tenant_id"`
	KeyID     string    `json:"key_id"`
	Nonce     string    `json:"nonce"`
	Timestamp int64     `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type FindBridgeAuthKey struct {
	TenantID int32
	KeyID    *string
	Status   *string
}

func (s *Store) CreateBridgeAuthKey(ctx context.Context, key *BridgeAuthKey) (*BridgeAuthKey, error) {
	return s.driver.CreateBridgeAuthKey(ctx, key)
}

func (s *Store) GetBridgeAuthKey(ctx context.Context, tenantID int32, keyID string) (*BridgeAuthKey, error) {
	return s.driver.GetBridgeAuthKey(ctx, tenantID, keyID)
}

func (s *Store) ListBridgeAuthKeys(ctx context.Context, tenantID int32) ([]*BridgeAuthKey, error) {
	return s.driver.ListBridgeAuthKeys(ctx, tenantID)
}

func (s *Store) UpdateBridgeAuthKeyLastUsed(ctx context.Context, tenantID int32, keyID string, now time.Time) error {
	return s.driver.UpdateBridgeAuthKeyLastUsed(ctx, tenantID, keyID, now)
}

func (s *Store) RevokeBridgeAuthKey(ctx context.Context, tenantID int32, keyID string, now time.Time) error {
	return s.driver.RevokeBridgeAuthKey(ctx, tenantID, keyID, now)
}

func (s *Store) StoreBridgeAuthNonce(ctx context.Context, nonce *BridgeAuthNonce) error {
	return s.driver.StoreBridgeAuthNonce(ctx, nonce)
}

func (s *Store) CleanupBridgeAuthNonces(ctx context.Context, now time.Time) error {
	return s.driver.CleanupBridgeAuthNonces(ctx, now)
}

