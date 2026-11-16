package mypage

import (
	"context"
	"regexp"
	"time"

	"firebase.google.com/go/v4/auth"
)

// FirebaseAuthClientは公式Admin SDKが実装する。
// コンストラクターは固定プロジェクトと鍵を使わないIAM署名者を使い、実サーバーではエミュレーターを使わない。
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

// tokenIssuedAtClockSkewは、発行時刻が未来であることを許す最大の時計ずれ。
// Google側の発行時刻とCloud Runの時計が秒境界をまたいでずれても、発行直後のトークンを拒否しないために使う。
// ID Tokenのiatとauth_timeが未来かどうかの比較だけに適用し、有効期限や署名などの検証は緩めない。
// App Checkのiatは公式SDK(golang-jwt)が許容なしで先に検証するため、ここでは緩和できない。
// Admin SDKのID Token検証は300秒を許容するが、本境界は追加検証なのでより狭い5秒に抑える。
const tokenIssuedAtClockSkew = 5 * time.Second

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
	latestIssuedAt := now + int64(tokenIssuedAtClockSkew/time.Second)
	if token.Audience != v.ProjectID || token.Issuer != "https://securetoken.google.com/"+v.ProjectID || token.Firebase.SignInProvider != "custom" || token.Firebase.Tenant != "" || !youtubeChannelID.MatchString(token.UID) || token.Subject != token.UID || token.Expires <= now || token.IssuedAt <= 0 || token.IssuedAt > latestIssuedAt || token.AuthTime <= 0 || token.AuthTime > latestIssuedAt || ctx.Err() != nil {
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
