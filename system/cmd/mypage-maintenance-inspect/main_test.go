package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app.modules/core/mypage"
)

func TestConfigurationRejectsBeforeCredentialDiscovery(t *testing.T) {
	valid := map[string]string{"GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "MYPAGE_ENVIRONMENT": "development", "PORT": "8080"}
	get := func(name string) string { return valid[name] }
	if project, environment, port, err := configuration(nil, get); err != nil || project != "test-youtube-study-space" || environment != "development" || port != "8080" {
		t.Fatalf("valid configuration rejected: %q %q %q %v", project, environment, port, err)
	}
	for name, value := range map[string]string{
		"GOOGLE_APPLICATION_CREDENTIALS": "/tmp/key.json",
		"FIRESTORE_EMULATOR_HOST":        "127.0.0.1:8080",
		"FIREBASE_AUTH_EMULATOR_HOST":    "127.0.0.1:9099",
		"GOOGLE_CLOUD_PROJECT":           "youtube-study-space",
		"MYPAGE_ENVIRONMENT":             "other",
		"PORT":                           "9090",
	} {
		candidate := map[string]string{}
		for key, item := range valid {
			candidate[key] = item
		}
		candidate[name] = value
		if _, _, _, err := configuration(nil, func(key string) string { return candidate[key] }); !errors.Is(err, errInspect) {
			t.Fatalf("%s was not rejected: %v", name, err)
		}
	}
	if _, _, _, err := configuration([]string{"--path", "other"}, get); !errors.Is(err, errInspect) {
		t.Fatalf("arguments were not rejected: %v", err)
	}
}

func TestInspectionHandlerReturnsOnlySanitizedReceipt(t *testing.T) {
	now := time.Date(2026, 10, 27, 2, 0, 0, 123456000, time.UTC)
	handler := inspectionHandler(func(context.Context) (mypage.MaintenanceInspection, error) {
		return mypage.MaintenanceInspection{
			Environment: "development", State: mypage.MaintenanceActive, Revision: 42,
			UpdatedAt: now, MessageTemplate: mypage.MaintenanceMessageTemplate,
			ReasonCategory: "PLANNED_MAINTENANCE", AuthoritativeReadbackVerified: true, ReadOnly: true,
		}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "https://service.invalid/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected response: %d %#v", response.Code, response.Header())
	}
	var receipt map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{"environment": true, "state": true, "revision": true, "updatedAt": true, "messageTemplate": true, "reasonCategory": true, "authoritativeReadbackVerified": true, "readOnly": true}
	if len(receipt) != len(expected) {
		t.Fatalf("unexpected receipt fields: %#v", receipt)
	}
	for key := range receipt {
		if !expected[key] {
			t.Fatalf("unexpected receipt field %q", key)
		}
	}
}

func TestInspectionHandlerFailsClosedWithoutErrorDetails(t *testing.T) {
	handler := inspectionHandler(func(context.Context) (mypage.MaintenanceInspection, error) {
		return mypage.MaintenanceInspection{}, errors.New("private firestore path and payload")
	})
	for name, target := range map[string][]string{
		"dependency": {http.MethodGet, "https://service.invalid/", ""},
		"method":     {http.MethodPost, "https://service.invalid/", ""},
		"query":      {http.MethodGet, "https://service.invalid/?path=other", ""},
		"body":       {http.MethodGet, "https://service.invalid/", "selector"},
	} {
		request := httptest.NewRequest(target[0], target[1], strings.NewReader(target[2]))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusServiceUnavailable && response.Code != http.StatusBadRequest {
			t.Fatalf("%s returned %d", name, response.Code)
		}
		if strings.Contains(response.Body.String(), "firestore") || strings.Contains(response.Body.String(), "payload") {
			t.Fatalf("%s leaked dependency details: %s", name, response.Body.String())
		}
	}
}
