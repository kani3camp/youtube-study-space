package mypage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/serviceaccess"
)

// confirmChainは、通常ログインのconfirmが呼ぶ依存先を呼び出し順に記録する。
type confirmChain struct {
	mu    sync.Mutex
	calls []string
	// 各依存先のn回目(1始まり)の呼び出しで返す結果。
	maintenanceActiveAt int
	accessRestrictedAt  int
	consumeErr          error
	tx                  OAuthTransaction
}

func (c *confirmChain) record(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, name)
	n := 0
	for _, call := range c.calls {
		if call == name {
			n++
		}
	}
	return n
}

func (c *confirmChain) recorded() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

type confirmChainStore struct {
	AuthStore
	chain *confirmChain
}

func (s *confirmChainStore) ReadVerified(context.Context, string, time.Time) (OAuthTransaction, error) {
	s.chain.record("read_verified")
	return s.chain.tx, nil
}

func (s *confirmChainStore) Consume(context.Context, string, string, string, Policy, time.Time) (Channel, error) {
	s.chain.record("consume")
	if s.chain.consumeErr != nil {
		return Channel{}, s.chain.consumeErr
	}
	return s.chain.tx.Channel, nil
}

type confirmChainMinter struct{ chain *confirmChain }

func (m confirmChainMinter) Mint(context.Context, string) (string, error) {
	m.chain.record("mint")
	return "synthetic-custom-token", nil
}

func confirmChainService(chain *confirmChain) *AuthService {
	now := nowForAccessTest()
	chain.tx = OAuthTransaction{Purpose: "login", Status: "channel_verified", ExpiresAt: now.Add(10 * time.Minute), ConfirmationRef: strings.Repeat("c", 64), Channel: Channel{ID: "UCsynthetic0000000000001", DisplayName: "Sample Channel"}}
	return &AuthService{
		Store: &confirmChainStore{chain: chain},
		Maintenance: &MaintenanceGate{readControl: func(context.Context) (MaintenanceControl, error) {
			n := chain.record("maintenance")
			return MaintenanceControl{Enabled: n == chain.maintenanceActiveAt, UpdatedAt: now}, nil
		}},
		Access: accessReaderFunc(func(context.Context, string) (serviceaccess.Snapshot, error) {
			if chain.record("access") == chain.accessRestrictedAt {
				return controlFixture(now, false), nil
			}
			return serviceaccess.Snapshot{}, nil
		}),
		Minter: confirmChainMinter{chain: chain},
		Policy: Policy{Privacy: "p1", Terms: "t1"},
		Now:    func() time.Time { return now },
	}
}

// /confirmの8秒予算内の依存先呼び出しを、確認済みトランザクションの読み取り1回を含む8回に固定する。
func TestConfirmLoginReadsVerifiedTransactionOncePerRequest(t *testing.T) {
	chain := &confirmChain{}
	h, _, _, _ := boundaryFixture()
	h.Auth = confirmChainService(chain)
	r := newBoundaryRequest(h, "POST", "/api/auth/youtube/confirm", `{"confirmationRef":"`+chain.tx.ConfirmationRef+`"}`)
	r.Header.Del("Authorization")
	r.AddCookie(&http.Cookie{Name: "__session", Value: strings.Repeat("a", 64)})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"purpose":"login","customToken":"synthetic-custom-token"}`, w.Body.String())
	require.Equal(t, []string{"read_verified", "maintenance", "access", "consume", "access", "maintenance", "mint", "access"}, chain.recorded())
}

// 読み取り済みの値を渡しても、消費前後のメンテナンス・アクセス確認とConsumeの再検証が発行を止める。
func TestConfirmResultGatesStillBlockTokenIssuance(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*confirmChain)
		code  string
		calls []string
	}{
		{"maintenance before consume", func(c *confirmChain) { c.maintenanceActiveAt = 1 }, "MAINTENANCE_ACTIVE", []string{"read_verified", "maintenance"}},
		{"restriction before consume", func(c *confirmChain) { c.accessRestrictedAt = 1 }, "SERVICE_ACCESS_RESTRICTED", []string{"read_verified", "maintenance", "access"}},
		{"transaction changed before consume", func(c *confirmChain) { c.consumeErr = apiError("OAUTH_TRANSACTION_CHANGED") }, "OAUTH_TRANSACTION_CHANGED", []string{"read_verified", "maintenance", "access", "consume"}},
		{"restriction after consume", func(c *confirmChain) { c.accessRestrictedAt = 2 }, "SERVICE_ACCESS_RESTRICTED", []string{"read_verified", "maintenance", "access", "consume", "access"}},
		{"maintenance after consume", func(c *confirmChain) { c.maintenanceActiveAt = 2 }, "MAINTENANCE_ACTIVE", []string{"read_verified", "maintenance", "access", "consume", "access", "maintenance"}},
		{"restriction during mint", func(c *confirmChain) { c.accessRestrictedAt = 3 }, "SERVICE_ACCESS_RESTRICTED", []string{"read_verified", "maintenance", "access", "consume", "access", "maintenance", "mint", "access"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := &confirmChain{}
			service := confirmChainService(chain)
			tc.setup(chain)
			response, err := service.ConfirmResult(context.Background(), strings.Repeat("a", 64), chain.tx.ConfirmationRef)
			require.Equal(t, tc.code, errorCode(err))
			require.Empty(t, response.CustomToken)
			require.Equal(t, tc.calls, chain.recorded())
		})
	}
}
