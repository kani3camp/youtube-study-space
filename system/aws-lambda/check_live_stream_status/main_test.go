package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"app.modules/aws-lambda/lambdautils"
	"google.golang.org/api/option"
)

type mockCheckLiveStreamApp struct {
	checkErr           error
	notifyTimeoutErr   error
	messageToOwnerMsgs []string
	closed             bool
}

func (m *mockCheckLiveStreamApp) CheckLiveStreamStatus(ctx context.Context) error {
	return m.checkErr
}

func (m *mockCheckLiveStreamApp) NotifyTimeoutToOwner(ctx context.Context, err error) error {
	if m.notifyTimeoutErr != nil {
		return m.notifyTimeoutErr
	}
	return nil
}

func (m *mockCheckLiveStreamApp) MessageToOwnerWithError(ctx context.Context, message string, err error) {
	m.messageToOwnerMsgs = append(m.messageToOwnerMsgs, message+": "+err.Error())
}

func (m *mockCheckLiveStreamApp) CloseFirestoreClient() {
	m.closed = true
}

func TestCheckLiveStreamFirestoreInitFailureReturnsOK(t *testing.T) {
	restore := stubCheckLiveStreamDeps(t, errors.New("cred"), nil, nil)
	defer restore()

	resp, err := CheckLiveStream(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
}

func TestCheckLiveStreamWorkspaceInitFailureReturnsOK(t *testing.T) {
	restore := stubCheckLiveStreamDeps(t, nil, errors.New("init failed"), nil)
	defer restore()

	resp, err := CheckLiveStream(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
}

func TestCheckLiveStreamHandlerFailureReturnsOKAfterOwnerMessage(t *testing.T) {
	app := &mockCheckLiveStreamApp{
		checkErr: errors.New("youtube api down"),
	}
	restore := stubCheckLiveStreamDeps(t, nil, nil, app)
	defer restore()

	resp, err := CheckLiveStream(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if len(app.messageToOwnerMsgs) != 1 {
		t.Fatalf("expected one owner message, got %#v", app.messageToOwnerMsgs)
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient to be called")
	}
}

func TestCheckLiveStreamTimeoutNotifyFailureReturnsError(t *testing.T) {
	app := &mockCheckLiveStreamApp{
		checkErr:         context.DeadlineExceeded,
		notifyTimeoutErr: errors.New("notify failed"),
	}
	restore := stubCheckLiveStreamDeps(t, nil, nil, app)
	defer restore()

	resp, err := CheckLiveStream(context.Background())
	if err == nil {
		t.Fatal("expected error when timeout notification fails")
	}
	if resp != (CheckLiveStreamResponse{}) {
		t.Fatalf("expected empty response, got %#v", resp)
	}
	if !strings.Contains(err.Error(), "timeout notification failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func stubCheckLiveStreamDeps(
	t *testing.T,
	firestoreErr error,
	newAppErr error,
	app checkLiveStreamApp,
) func() {
	t.Helper()
	origF := firestoreClientOptionCheck
	origN := newCheckWorkspaceApp

	firestoreClientOptionCheck = func() (option.ClientOption, error) {
		return option.WithoutAuthentication(), firestoreErr
	}
	newCheckWorkspaceApp = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (checkLiveStreamApp, error) {
		if newAppErr != nil {
			return nil, newAppErr
		}
		return app, nil
	}

	return func() {
		firestoreClientOptionCheck = origF
		newCheckWorkspaceApp = origN
	}
}
