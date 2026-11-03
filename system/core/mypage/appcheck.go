package mypage

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/appcheck"
	"google.golang.org/api/option"
)

var projectNumberPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

// FirebaseAppCheckClientは公式SDKが署名・標準claim・JWKSを検証する境界。
type FirebaseAppCheckClient interface {
	VerifyToken(string) (*appcheck.DecodedAppCheckToken, error)
}

// NewFirebaseAppCheckClientは起動時にだけ呼ぶ。ctxは初回公開JWKS取得の期限を制限する。
// 定期更新は公式SDKがプロセス内で管理する。
// App Checkのaudienceはproject numberを使うため、Authとは別のAppに固定する。
// 署名鍵は公開JWKSから得る。ADCやサービスアカウント鍵は不要。
func NewFirebaseAppCheckClient(ctx context.Context, projectNumber string) (FirebaseAppCheckClient, error) {
	if !projectNumberPattern.MatchString(projectNumber) || ctx.Err() != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectNumber}, option.WithoutAuthentication())
	if err != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	client, err := app.AppCheck(ctx)
	if err != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	return client, nil
}

type AppCheckVerifier struct {
	projectNumber, appID string
	client               FirebaseAppCheckClient
	now                  func() time.Time
}

func validAppCheckConfig(projectNumber, appID string) bool {
	return projectNumberPattern.MatchString(projectNumber) && len(appID) <= 256 && strings.HasPrefix(appID, "1:"+projectNumber+":web:") && len(appID) > len("1:"+projectNumber+":web:")
}

func NewAppCheckVerifier(projectNumber, appID string, client FirebaseAppCheckClient, now func() time.Time) (*AppCheckVerifier, error) {
	if !validAppCheckConfig(projectNumber, appID) || client == nil || now == nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	return &AppCheckVerifier{projectNumber: projectNumber, appID: appID, client: client, now: now}, nil
}

func (v *AppCheckVerifier) Verify(ctx context.Context, raw string) (resultErr error) {
	// SDK v4.22の署名済みclaimの型assertion panicも認証拒否へ変換する。
	// トークン・依存先エラー・panic値はログや公開応答へ出さない。
	defer func() {
		if recover() != nil {
			resultErr = apiError("APP_CHECK_REQUIRED")
		}
	}()
	if v == nil || ctx.Err() != nil || raw == "" || len(raw) > 16384 {
		return apiError("APP_CHECK_REQUIRED")
	}
	token, err := v.client.VerifyToken(raw)
	if err != nil || token == nil || ctx.Err() != nil {
		return apiError("APP_CHECK_REQUIRED")
	}
	now := v.now()
	// SDKはissuer prefixと非空subjectだけを検査する。ここでMyPageの環境・Web Appを固定する。
	if token.Issuer != "https://firebaseappcheck.googleapis.com/"+v.projectNumber || !slices.Contains(token.Audience, "projects/"+v.projectNumber) || token.AppID != v.appID || token.Subject != v.appID || token.IssuedAt.Unix() <= 0 || token.IssuedAt.After(now) || !now.Before(token.ExpiresAt) {
		return apiError("APP_CHECK_REQUIRED")
	}
	return nil
}
