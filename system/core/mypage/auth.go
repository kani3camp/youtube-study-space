package mypage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"app.modules/core/serviceaccess"
)

// APIError carries only a stable allowlisted code. Dependency errors are never
// used as public messages or normal log fields.
type APIError struct{ Code string }

func (e *APIError) Error() string { return e.Code }
func apiError(code string) error  { return &APIError{Code: code} }
func errorCode(err error) string {
	var e *APIError
	if errors.As(err, &e) {
		switch e.Code {
		case "INVALID_REQUEST", "PAYLOAD_TOO_LARGE", "PRIVACY_CONSENT_REQUIRED", "POLICY_VERSION_OUTDATED", "AUTH_REQUIRED", "APP_CHECK_REQUIRED", "SERVICE_ACCESS_RESTRICTED", "DATA_DELETION_IN_PROGRESS", "OAUTH_TRANSACTION_REQUIRED", "OAUTH_TRANSACTION_PENDING", "OAUTH_TRANSACTION_EXPIRED", "OAUTH_TRANSACTION_CONSUMED", "OAUTH_TRANSACTION_CHANGED", "PRIVACY_RECONSENT_REQUIRED", "SUPPORT_CHALLENGE_INVALID", "SUPPORT_CHANNEL_MISMATCH", "INTAKE_KEY_CONFLICT", "OAUTH_FAILED", "OAUTH_SCOPE_INSUFFICIENT", "CHANNEL_UNAVAILABLE", "CHANNEL_AMBIGUOUS", "WEB_ACCOUNT_REQUIRED", "RATE_LIMITED", "TEMPORARY_UNAVAILABLE", "INTERNAL_ERROR":
			return e.Code
		}
	}
	return "INTERNAL_ERROR"
}

type Policy struct {
	Privacy string
	Terms   string
}

type Channel struct {
	ID          string  `firestore:"channelId"`
	DisplayName string  `firestore:"displayName"`
	Handle      *string `firestore:"handle"`
	AvatarURL   *string `firestore:"avatarUrl"`
}

type OAuthTransaction struct {
	Purpose              string          `firestore:"purpose"`
	Support              *SupportBinding `firestore:"support"`
	StateHash            string          `firestore:"stateHash"`
	Status               string          `firestore:"status"`
	PrivacyPolicyVersion string          `firestore:"privacyPolicyVersion"`
	TermsVersion         string          `firestore:"termsVersion"`
	PrivacyConsentedAt   time.Time       `firestore:"privacyConsentedAt"`
	CreatedAt            time.Time       `firestore:"createdAt"`
	ExpiresAt            time.Time       `firestore:"expiresAt"`
	ConfirmationRef      string          `firestore:"confirmationRef"`
	Channel              Channel         `firestore:"channel"`
	VerifiedAt           time.Time       `firestore:"verifiedAt"`
	ConsumedAt           time.Time       `firestore:"consumedAt"`
}

type WebAccount struct {
	Revision         time.Time `firestore:"-"`
	AccessCheckpoint string    `firestore:"-"`
	// AccessBlocked is decoded for compatibility only; ServiceAccessControl is authoritative.
	AccessBlocked        bool       `firestore:"accessBlocked"`
	PrivacyPolicyVersion string     `firestore:"privacyPolicyVersion"`
	TermsVersion         string     `firestore:"termsVersion"`
	PrivacyConsentedAt   time.Time  `firestore:"privacyConsentedAt"`
	FirstWebLoginAt      *time.Time `firestore:"firstWebLoginAt"`
	DisplayName          string     `firestore:"displayName"`
	Handle               *string    `firestore:"handle"`
	AvatarURL            *string    `firestore:"avatarUrl"`
	MetadataFetchedAt    time.Time  `firestore:"metadataFetchedAt"`
	CreatedAt            time.Time  `firestore:"createdAt"`
	UpdatedAt            time.Time  `firestore:"updatedAt"`
}

// AuthStore methods are atomic; no external provider call belongs in their
// transaction callbacks. Neither store contains OAuth or Firebase tokens.
type AuthStore interface {
	Create(context.Context, string, string, OAuthTransaction, time.Time) error
	Claim(context.Context, string, string, time.Time) error
	Verify(context.Context, string, Channel, string, time.Time) error
	Fail(context.Context, string) error
	ReadVerified(context.Context, string, time.Time) (OAuthTransaction, error)
	Consume(context.Context, string, string, string, Policy, time.Time) (Channel, error)
	ReadAccount(context.Context, string) (WebAccount, error)
	CompleteSession(context.Context, string, string, Policy, time.Time) error
}

// Provider implementations discard the exchanged OAuth token before returning
// verified channels and must check youtube.readonly in the granted scope.
type YouTubeOAuth interface {
	AuthorizationURL(state string) string
	Resolve(context.Context, string) ([]Channel, error)
}

type CustomTokenMinter interface {
	Mint(context.Context, string) (string, error)
}

type AuthService struct {
	Access   serviceaccess.Reader
	Store    AuthStore
	Provider YouTubeOAuth
	Minter   CustomTokenMinter
	Policy   Policy
	Support  *FirestoreSupportStore
	Now      func() time.Time
}

type StartRequest struct {
	SupportChallenge     *string `json:"supportChallenge,omitempty"`
	PrivacyPolicyVersion string  `json:"privacyPolicyVersion"`
	PrivacyAccepted      bool    `json:"privacyAccepted"`
	TermsVersion         string  `json:"termsVersion"`
	TermsAccepted        bool    `json:"termsAccepted"`
}

type StartResponse struct {
	AuthorizationURL string    `json:"authorizationUrl"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type ChannelResponse struct {
	Purpose         string         `json:"purpose"`
	SupportPurpose  SupportPurpose `json:"supportPurpose,omitempty"`
	DisplayName     string         `json:"displayName"`
	Handle          *string        `json:"handle"`
	AvatarURL       *string        `json:"avatarUrl"`
	ConfirmationRef string         `json:"confirmationRef"`
}

func opaque() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate opaque reference: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *AuthService) Start(ctx context.Context, previousID string, request StartRequest) (StartResponse, string, error) {
	if !request.PrivacyAccepted || !request.TermsAccepted {
		return StartResponse{}, "", apiError("PRIVACY_CONSENT_REQUIRED")
	}
	if s.Policy.Privacy == "" || s.Policy.Terms == "" {
		return StartResponse{}, "", apiError("INTERNAL_ERROR")
	}
	if request.PrivacyPolicyVersion != s.Policy.Privacy || request.TermsVersion != s.Policy.Terms {
		return StartResponse{}, "", apiError("POLICY_VERSION_OUTDATED")
	}
	id, err := opaque()
	if err != nil {
		return StartResponse{}, "", err
	}
	state, err := opaque()
	if err != nil {
		return StartResponse{}, "", err
	}
	now := s.Now().UTC()
	tx := OAuthTransaction{Purpose: "login", StateHash: digest(state), Status: "pending", PrivacyPolicyVersion: request.PrivacyPolicyVersion, TermsVersion: request.TermsVersion, PrivacyConsentedAt: now, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute)}
	var storeErr error
	if request.SupportChallenge != nil {
		if s.Support == nil || !validOpaque(*request.SupportChallenge) {
			return StartResponse{}, "", apiError("SUPPORT_CHALLENGE_INVALID")
		}
		binding, err := s.Support.Resolve(ctx, *request.SupportChallenge, now)
		if err != nil {
			return StartResponse{}, "", err
		}
		store, ok := s.Store.(supportAuthStore)
		if !ok {
			return StartResponse{}, "", apiError("TEMPORARY_UNAVAILABLE")
		}
		tx.Purpose = "support"
		tx.Support = &binding
		storeErr = store.CreateSupport(ctx, id, previousID, tx, s.Support.Environment, now)
	} else {
		storeErr = s.Store.Create(ctx, id, previousID, tx, now)
	}
	if err := storeErr; err != nil {
		return StartResponse{}, "", fmt.Errorf("start OAuth transaction: %w", err)
	}
	return StartResponse{AuthorizationURL: s.Provider.AuthorizationURL(state), ExpiresAt: tx.ExpiresAt}, id, nil
}

func (s *AuthService) Callback(ctx context.Context, id, state, code string, denied bool) error {
	if id == "" || state == "" {
		return apiError("OAUTH_TRANSACTION_REQUIRED")
	}
	if err := s.Store.Claim(ctx, id, digest(state), s.Now().UTC()); err != nil {
		return fmt.Errorf("claim OAuth callback: %w", err)
	}
	if denied {
		if err := s.Store.Fail(ctx, id); err != nil {
			return fmt.Errorf("fail denied OAuth transaction: %w", err)
		}
		return apiError("OAUTH_FAILED")
	}
	channels, err := s.Provider.Resolve(ctx, code)
	if err != nil {
		if failErr := s.Store.Fail(ctx, id); failErr != nil {
			return fmt.Errorf("fail OAuth transaction: %w", failErr)
		}
		return fmt.Errorf("resolve OAuth channel: %w", err)
	}
	if len(channels) != 1 || channels[0].ID == "" || channels[0].DisplayName == "" {
		if err := s.Store.Fail(ctx, id); err != nil {
			return fmt.Errorf("fail unavailable channel: %w", err)
		}
		if len(channels) > 1 {
			return apiError("CHANNEL_AMBIGUOUS")
		}
		return apiError("CHANNEL_UNAVAILABLE")
	}
	confirmation, err := opaque()
	if err != nil {
		return err
	}
	if err := s.Store.Verify(ctx, id, channels[0], confirmation, s.Now().UTC()); err != nil {
		return fmt.Errorf("verify OAuth transaction: %w", err)
	}
	return nil
}

func (s *AuthService) Channel(ctx context.Context, id string) (ChannelResponse, error) {
	tx, err := s.Store.ReadVerified(ctx, id, s.Now().UTC())
	if err != nil {
		return ChannelResponse{}, fmt.Errorf("read verified channel: %w", err)
	}
	purpose := transactionPurpose(tx)
	if purpose == "support" {
		if s.Support == nil || tx.Support == nil {
			return ChannelResponse{}, apiError("SUPPORT_CHALLENGE_INVALID")
		}
		if err := s.Support.CheckBinding(ctx, *tx.Support, tx.Channel.ID, s.Now().UTC()); err != nil {
			return ChannelResponse{}, err
		}
		return ChannelResponse{Purpose: purpose, SupportPurpose: tx.Support.Purpose, DisplayName: tx.Channel.DisplayName, Handle: tx.Channel.Handle, AvatarURL: tx.Channel.AvatarURL, ConfirmationRef: tx.ConfirmationRef}, nil
	}
	if purpose != "login" {
		return ChannelResponse{}, apiError("INVALID_REQUEST")
	}
	if _, err := readAccess(ctx, s.Access, tx.Channel.ID, ""); err != nil {
		return ChannelResponse{}, err
	}
	return ChannelResponse{Purpose: purpose, DisplayName: tx.Channel.DisplayName, Handle: tx.Channel.Handle, AvatarURL: tx.Channel.AvatarURL, ConfirmationRef: tx.ConfirmationRef}, nil
}

func (s *AuthService) Confirm(ctx context.Context, id, confirmation string) (string, error) {
	if confirmation == "" {
		return "", apiError("INVALID_REQUEST")
	}
	value, err := s.Store.ReadVerified(ctx, id, s.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("read confirmed channel: %w", err)
	}
	checkpoint, err := readAccess(ctx, s.Access, value.Channel.ID, "")
	if err != nil {
		return "", err
	}
	channel, err := s.Store.Consume(ctx, id, confirmation, checkpoint, s.Policy, s.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("consume confirmed channel: %w", err)
	}
	// Consume is committed before mint. A timeout or failed mint cannot be retried
	// from this transaction, even when the provider may have issued a token.
	if _, err := readAccess(ctx, s.Access, channel.ID, checkpoint); err != nil {
		return "", err
	}
	token, err := s.Minter.Mint(ctx, channel.ID)
	if err != nil {
		return "", apiError("TEMPORARY_UNAVAILABLE")
	}
	if _, err := readAccess(ctx, s.Access, channel.ID, checkpoint); err != nil {
		return "", err
	}
	return token, nil
}

func checkTransaction(tx OAuthTransaction, now time.Time, expected string) error {
	if !now.Before(tx.ExpiresAt) {
		return apiError("OAUTH_TRANSACTION_EXPIRED")
	}
	if tx.Status == "consumed" {
		return apiError("OAUTH_TRANSACTION_CONSUMED")
	}
	if tx.Status != expected {
		return apiError("OAUTH_TRANSACTION_PENDING")
	}
	return nil
}

func checkPolicy(account WebAccount, policy Policy) error {
	if account.PrivacyPolicyVersion != policy.Privacy || account.TermsVersion != policy.Terms {
		return apiError("PRIVACY_RECONSENT_REQUIRED")
	}
	return nil
}

type supportAuthStore interface {
	CreateSupport(context.Context, string, string, OAuthTransaction, string, time.Time) error
	ConsumeSupport(context.Context, string, string, Policy, string, string, time.Time) error
}

type ConfirmResponse struct {
	Purpose     string `json:"purpose"`
	CustomToken string `json:"customToken,omitempty"`
	RequestRef  string `json:"requestRef,omitempty"`
}

func transactionPurpose(tx OAuthTransaction) string {
	if tx.Purpose == "" && tx.Support == nil {
		return "login"
	}
	return tx.Purpose
}

func (s *AuthService) ConfirmResult(ctx context.Context, id, confirmation string) (ConfirmResponse, error) {
	tx, err := s.Store.ReadVerified(ctx, id, s.Now().UTC())
	if err != nil {
		return ConfirmResponse{}, fmt.Errorf("read confirmation purpose: %w", err)
	}
	if transactionPurpose(tx) == "login" {
		token, err := s.Confirm(ctx, id, confirmation)
		if err != nil {
			return ConfirmResponse{}, err
		}
		return ConfirmResponse{Purpose: "login", CustomToken: token}, nil
	}
	store, ok := s.Store.(supportAuthStore)
	if !ok || s.Support == nil || transactionPurpose(tx) != "support" || tx.Support == nil {
		return ConfirmResponse{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	proofRef, err := opaque()
	if err != nil {
		return ConfirmResponse{}, err
	}
	if err := store.ConsumeSupport(ctx, id, confirmation, s.Policy, s.Support.Environment, proofRef, s.Now().UTC()); err != nil {
		return ConfirmResponse{}, fmt.Errorf("confirm support purpose: %w", err)
	}
	return ConfirmResponse{Purpose: "support", RequestRef: proofRef}, nil
}
