package mypage

import (
	"context"
	"regexp"
	"time"

	"firebase.google.com/go/v4/auth"
)

// FirebaseAuthClient is implemented by the official Admin SDK. Its constructor
// must use a fixed project and keyless IAM signer; no emulator in a real server.
type FirebaseAuthClient interface {
	VerifyIDToken(context.Context, string) (*auth.Token, error)
	CustomToken(context.Context, string) (string, error)
}

type FirebaseBoundary struct {
	Client    FirebaseAuthClient
	AppCheck  *AppCheckVerifier
	ProjectID string
	Now       func() time.Time
}

var firebaseProjectID = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

func (v *FirebaseBoundary) VerifyIDToken(ctx context.Context, raw string) (VerifiedIdentity, error) {
	if v.Client == nil || v.Now == nil || !firebaseProjectID.MatchString(v.ProjectID) || raw == "" || len(raw) > 16384 {
		return VerifiedIdentity{}, apiError("AUTH_REQUIRED")
	}
	token, err := v.Client.VerifyIDToken(ctx, raw)
	if err != nil || token == nil {
		return VerifiedIdentity{}, apiError("AUTH_REQUIRED")
	}
	now := v.Now().Unix()
	if token.Audience != v.ProjectID || token.Issuer != "https://securetoken.google.com/"+v.ProjectID || token.Firebase.SignInProvider != "custom" || token.Firebase.Tenant != "" || !youtubeChannelID.MatchString(token.UID) || token.Subject != token.UID || token.Expires <= now || token.IssuedAt <= 0 || token.IssuedAt > now || token.AuthTime <= 0 || token.AuthTime > now || ctx.Err() != nil {
		return VerifiedIdentity{}, apiError("AUTH_REQUIRED")
	}
	return VerifiedIdentity{UID: token.UID, Provider: "custom"}, nil
}

func (v *FirebaseBoundary) VerifyAppCheck(ctx context.Context, raw string) error {
	if v.AppCheck == nil {
		return apiError("APP_CHECK_REQUIRED")
	}
	return v.AppCheck.Verify(ctx, raw)
}

func (v *FirebaseBoundary) Mint(ctx context.Context, uid string) (string, error) {
	if v.Client == nil || !firebaseProjectID.MatchString(v.ProjectID) || !youtubeChannelID.MatchString(uid) || ctx.Err() != nil {
		return "", apiError("TEMPORARY_UNAVAILABLE")
	}
	token, err := v.Client.CustomToken(ctx, uid)
	if err != nil || token == "" || ctx.Err() != nil {
		return "", apiError("TEMPORARY_UNAVAILABLE")
	}
	return token, nil
}
