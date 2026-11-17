package mypage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestPublicMaintenanceStatusReturnsOnlyBoolean(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		h, _, _, _ := boundaryFixture()
		h.Maintenance = &MaintenanceGate{readControl: func(context.Context) (MaintenanceControl, error) {
			return MaintenanceControl{Enabled: enabled, UpdatedAt: time.Now()}, nil
		}}
		response := boundaryRequest(h, "GET", "/api/public/mypage-status", "")
		if response.Code != http.StatusOK || response.Body.String() != fmt.Sprintf("{\"maintenanceEnabled\":%t}\n", enabled) {
			t.Fatalf("enabled=%t status=%d body=%s", enabled, response.Code, response.Body.String())
		}
	}
	h, _, _, _ := boundaryFixture()
	h.Maintenance = &MaintenanceGate{readControl: func(context.Context) (MaintenanceControl, error) {
		return MaintenanceControl{}, errors.New("unavailable")
	}}
	response := boundaryRequest(h, "GET", "/api/public/mypage-status", "")
	if response.Code != http.StatusServiceUnavailable || responseCode(t, response) != "TEMPORARY_UNAVAILABLE" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMaintenanceProtectedAPI(t *testing.T) {
	for _, tt := range []struct {
		name    string
		enabled bool
		readErr error
		status  int
		code    string
	}{
		{name: "normal", status: http.StatusOK},
		{name: "active", enabled: true, status: http.StatusServiceUnavailable, code: "MAINTENANCE_ACTIVE"},
		{name: "read failure", readErr: apiError("TEMPORARY_UNAVAILABLE"), status: http.StatusServiceUnavailable, code: "TEMPORARY_UNAVAILABLE"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _, reads := boundaryFixture()
			calls := 0
			h.Maintenance = &MaintenanceGate{readControl: func(context.Context) (MaintenanceControl, error) {
				calls++
				return MaintenanceControl{Enabled: tt.enabled, UpdatedAt: time.Now()}, tt.readErr
			}}
			w := boundaryRequest(h, "GET", "/api/mypage", "")
			if w.Code != tt.status {
				t.Fatalf("status %d, body %s", w.Code, w.Body.String())
			}
			if tt.code != "" && (responseCode(t, w) != tt.code || *reads != 0) {
				t.Fatalf("body %s reads %d", w.Body.String(), *reads)
			}
			if calls != 1 {
				t.Fatalf("read calls %d", calls)
			}
		})
	}
}

func TestMaintenanceReadsEveryRequest(t *testing.T) {
	calls := 0
	gate := &MaintenanceGate{readControl: func(context.Context) (MaintenanceControl, error) {
		calls++
		if calls == 2 {
			return MaintenanceControl{}, errors.New("unavailable")
		}
		return MaintenanceControl{UpdatedAt: time.Now()}, nil
	}}
	if err := gate.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gate.Check(context.Background()); err == nil {
		t.Fatal("failed read was accepted")
	}
	if calls != 2 {
		t.Fatal("control read was cached")
	}
}

type maintenanceConfirmStore struct {
	AuthStore
	channel Channel
}

func (s *maintenanceConfirmStore) ReadVerified(context.Context, string, time.Time) (OAuthTransaction, error) {
	return OAuthTransaction{Purpose: "login", Channel: s.channel}, nil
}

func (s *maintenanceConfirmStore) Consume(context.Context, string, string, string, Policy, time.Time) (Channel, error) {
	return s.channel, nil
}

type maintenanceTestMinter struct{ calls int }

func (m *maintenanceTestMinter) Mint(context.Context, string) (string, error) {
	m.calls++
	return "token", nil
}

func TestMaintenanceRecheckedBeforeTokenMint(t *testing.T) {
	calls := 0
	gate := &MaintenanceGate{readControl: func(context.Context) (MaintenanceControl, error) {
		calls++
		return MaintenanceControl{Enabled: calls > 1, UpdatedAt: time.Now()}, nil
	}}
	minter := &maintenanceTestMinter{}
	service := &AuthService{Access: allowedAccess(), Store: &maintenanceConfirmStore{channel: Channel{ID: "synthetic"}}, Maintenance: gate, Minter: minter, Now: time.Now}
	token, err := service.Confirm(context.Background(), "transaction", "confirmation")
	if errorCode(err) != "MAINTENANCE_ACTIVE" || token != "" || minter.calls != 0 {
		t.Fatalf("token=%q err=%v mint calls=%d", token, err, minter.calls)
	}
}
