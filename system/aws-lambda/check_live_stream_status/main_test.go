package main

import (
	"context"
	"errors"
	"testing"

	"app.modules/aws-lambda/lambdautils"
	"google.golang.org/api/option"
)

type mockCheckLiveStreamApp struct {
	checkErr           error
	messageToOwnerMsgs []string
	closed             bool
}

func (m *mockCheckLiveStreamApp) CheckLiveStreamStatus(ctx context.Context) error {
	return m.checkErr
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

func TestCheckLiveStreamTimeoutReturnsOK(t *testing.T) {
	app := &mockCheckLiveStreamApp{
		checkErr: context.DeadlineExceeded,
	}
	restore := stubCheckLiveStreamDeps(t, nil, nil, app)
	defer restore()

	resp, err := CheckLiveStream(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on handled timeout, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient to be called")
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
