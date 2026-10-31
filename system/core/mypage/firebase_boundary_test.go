package mypage

import (
	"context"
	"errors"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/require"
)

type fakeFirebaseClient struct {
	token     *auth.Token
	err       error
	mintCalls int
	mintedUID string
}

func (f *fakeFirebaseClient) VerifyIDToken(context.Context, string) (*auth.Token, error) {
	return f.token, f.err
}

func (f *fakeFirebaseClient) CustomToken(_ context.Context, uid string) (string, error) {
	f.mintCalls++
	f.mintedUID = uid
	if f.err != nil {
		return "", f.err
	}
	return "synthetic-custom-token", nil
}

func TestFirebaseBoundaryAcceptsOnlyCurrentProjectCustomChannelIdentity(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	good := auth.Token{UID: "UCsynthetic0000000000001", Subject: "UCsynthetic0000000000001", Issuer: "https://securetoken.google.com/demo-mypage", Audience: "demo-mypage", Expires: now.Add(time.Hour).Unix(), IssuedAt: now.Unix(), AuthTime: now.Unix(), Firebase: auth.FirebaseInfo{SignInProvider: "custom"}}
	for _, tc := range []struct {
		name   string
		mutate func(*auth.Token)
	}{
		{"valid", func(*auth.Token) {}},
		{"wrong project", func(v *auth.Token) { v.Audience = "demo-other" }},
		{"wrong issuer", func(v *auth.Token) { v.Issuer = "https://securetoken.google.com/demo-other" }},
		{"old google login", func(v *auth.Token) { v.Firebase.SignInProvider = "google.com" }},
		{"tenant", func(v *auth.Token) { v.Firebase.Tenant = "synthetic-tenant" }},
		{"not channel uid", func(v *auth.Token) { v.UID = "../../synthetic" }},
		{"subject mismatch", func(v *auth.Token) { v.Subject = "UCsynthetic0000000000002" }},
		{"expiry boundary", func(v *auth.Token) { v.Expires = now.Unix() }},
		{"future issue", func(v *auth.Token) { v.IssuedAt = now.Add(time.Second).Unix() }},
		{"future auth", func(v *auth.Token) { v.AuthTime = now.Add(time.Second).Unix() }},
		{"missing issue", func(v *auth.Token) { v.IssuedAt = 0 }},
		{"missing auth", func(v *auth.Token) { v.AuthTime = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := good
			tc.mutate(&value)
			f := &fakeFirebaseClient{token: &value}
			v := &FirebaseBoundary{Client: f, ProjectID: "demo-mypage", Now: func() time.Time { return now }}
			identity, err := v.VerifyIDToken(context.Background(), "synthetic-id-token")
			if tc.name == "valid" {
				require.NoError(t, err)
				require.Equal(t, VerifiedIdentity{UID: good.UID, Provider: "custom"}, identity)
			} else {
				require.Equal(t, "AUTH_REQUIRED", err.Error())
				require.Empty(t, identity.UID)
			}
			require.Zero(t, f.mintCalls)
		})
	}
}

func TestFirebaseBoundaryRedactsErrorsAndOnlyMintsVerifiedChannelUID(t *testing.T) {
	f := &fakeFirebaseClient{err: errors.New("synthetic-secret synthetic-id-token")}
	v := &FirebaseBoundary{Client: f, ProjectID: "demo-mypage", Now: time.Now}
	_, err := v.VerifyIDToken(context.Background(), "synthetic-id-token")
	require.Equal(t, "AUTH_REQUIRED", err.Error())
	minted, err := v.Mint(context.Background(), "UCsynthetic0000000000001")
	require.Empty(t, minted)
	require.Equal(t, "TEMPORARY_UNAVAILABLE", err.Error())
	require.Equal(t, 1, f.mintCalls)
	f.err = nil
	minted, err = v.Mint(context.Background(), "UCsynthetic0000000000001")
	require.NoError(t, err)
	require.Equal(t, "synthetic-custom-token", minted)
	require.Equal(t, "UCsynthetic0000000000001", f.mintedUID)
	_, err = v.Mint(context.Background(), "../../synthetic")
	require.Error(t, err)
	require.Equal(t, 2, f.mintCalls)
	require.Equal(t, "APP_CHECK_REQUIRED", v.VerifyAppCheck(context.Background(), "synthetic-app-token").Error())
}
