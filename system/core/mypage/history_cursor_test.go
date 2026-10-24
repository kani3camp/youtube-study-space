package mypage

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHistoryCursorIsOpaqueAndBoundToIdentityAuthorizationAndQuery(t *testing.T) {
	c, err := newHistoryCipher([]byte("synthetic-existing-server-secret"), "development")
	require.NoError(t, err)
	q, err := parseHistoryQuery("from=2026-10-01&to=2026-11-01&limit=50")
	require.NoError(t, err)
	now := instant("2026-10-24T03:00:00Z")
	value := historyCursor{Owner: c.owner("synthetic-owner"), Authorization: "policy-access-tier-v1", From: "2026-10-01", To: "2026-11-01", PageSize: 50, AsOf: now, ExpiresAt: now.Add(historyCursorTTL), AfterEnd: now.Add(-time.Minute), AfterID: "private-source-document"}
	token, err := c.seal(value)
	require.NoError(t, err)
	encoded, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), value.AfterID)
	require.NotContains(t, string(encoded), "synthetic-owner")
	opened, err := c.open(token, "synthetic-owner", value.Authorization, q, now)
	require.NoError(t, err)
	require.Equal(t, value, opened)
	for _, fault := range []string{"owner", "authorization", "from", "limit", "tamper", "expired"} {
		t.Run(fault, func(t *testing.T) {
			uid, authorization, query, at, modified := "synthetic-owner", value.Authorization, q, now, token
			switch fault {
			case "owner":
				uid = "another-owner"
			case "authorization":
				authorization = "policy-access-tier-v2"
			case "from":
				query.From = query.From.AddDate(0, 0, 1)
			case "limit":
				query.PageSize = 100
			case "tamper":
				bytes := append([]byte(nil), encoded...)
				bytes[len(bytes)-1] ^= 1
				modified = base64.RawURLEncoding.EncodeToString(bytes)
			case "expired":
				at = value.ExpiresAt
			}
			_, err := c.open(modified, uid, authorization, query, at)
			require.Equal(t, "INVALID_CURSOR", errorCode(err))
		})
	}
}

func TestHistoryQueryRefusesOffsetsUIDPlanDuplicatesAndUnboundedRanges(t *testing.T) {
	for _, query := range []string{
		"from=2026-10-01&to=2026-11-02",
		"from=2026-10-01&to=2026-10-01",
		"from=2026-02-30&to=2026-03-01",
		"from=2026-10-01&to=2026-10-02&offset=50",
		"from=2026-10-01&to=2026-10-02&uid=other",
		"from=2026-10-01&to=2026-10-02&plan=premium",
		"from=2026-10-01&from=2026-10-02&to=2026-10-03",
		"from=2026-10-01&to=2026-10-02&limit=101",
		"from=2026-10-01&to=2026-10-02&limit=0",
		"from=2026-10-01&to=2026-10-02&cursor=" + strings.Repeat("x", 2049),
	} {
		_, err := parseHistoryQuery(query)
		require.Error(t, err, query)
	}
	q, err := parseHistoryQuery("from=2026-01-31&to=2026-03-03")
	require.NoError(t, err)
	require.Equal(t, 50, q.PageSize)
	require.Equal(t, 31*24*time.Hour, q.To.Sub(q.From))
}
