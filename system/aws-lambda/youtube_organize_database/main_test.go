package main

import (
	"context"
	"errors"
	"testing"

	"app.modules/aws-lambda/lambdautils"
	"google.golang.org/api/option"
)

type mockOrganizeDatabaseApp struct {
	organizeDBFunc      func(ctx context.Context, isMemberRoom bool) error
	messageToOwnerCalls []string
	closed              bool
}

func (m *mockOrganizeDatabaseApp) OrganizeDB(ctx context.Context, isMemberRoom bool) error {
	return m.organizeDBFunc(ctx, isMemberRoom)
}

func (m *mockOrganizeDatabaseApp) MessageToOwnerWithError(ctx context.Context, message string, err error) {
	m.messageToOwnerCalls = append(m.messageToOwnerCalls, message+": "+err.Error())
}

func (m *mockOrganizeDatabaseApp) CloseFirestoreClient() {
	m.closed = true
}

func TestOrganizeDatabaseSuccess(t *testing.T) {
	app := &mockOrganizeDatabaseApp{
		organizeDBFunc: func(ctx context.Context, isMemberRoom bool) error { return nil },
	}

	restore := stubOrganizeDatabaseDeps(t, app, nil, nil)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if !app.closed {
		t.Fatal("expected firestore client to be closed")
	}
}

func TestOrganizeDatabaseReturnsJoinedErrorAfterBothRoomsRun(t *testing.T) {
	memberErr := errors.New("member failed")
	generalErr := errors.New("general failed")
	var callOrder []bool
	app := &mockOrganizeDatabaseApp{
		organizeDBFunc: func(ctx context.Context, isMemberRoom bool) error {
			callOrder = append(callOrder, isMemberRoom)
			if isMemberRoom {
				return memberErr
			}
			return generalErr
		},
	}

	restore := stubOrganizeDatabaseDeps(t, app, nil, nil)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result after handled failures, got %#v", resp)
	}
	if len(callOrder) != 2 || callOrder[0] != true || callOrder[1] != false {
		t.Fatalf("expected member then general execution, got %#v", callOrder)
	}
	if len(app.messageToOwnerCalls) != 2 {
		t.Fatalf("expected 2 owner notifications, got %#v", app.messageToOwnerCalls)
	}
}

func TestOrganizeDatabaseContinuesToGeneralRoomAfterMemberFailure(t *testing.T) {
	memberErr := errors.New("member failed")
	var callOrder []bool
	app := &mockOrganizeDatabaseApp{
		organizeDBFunc: func(ctx context.Context, isMemberRoom bool) error {
			callOrder = append(callOrder, isMemberRoom)
			if isMemberRoom {
				return memberErr
			}
			return nil
		},
	}

	restore := stubOrganizeDatabaseDeps(t, app, nil, nil)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if len(callOrder) != 2 || callOrder[1] != false {
		t.Fatalf("expected general room to run after member failure, got %#v", callOrder)
	}
	if len(app.messageToOwnerCalls) != 1 {
		t.Fatalf("expected one owner notification, got %#v", app.messageToOwnerCalls)
	}
}

func TestOrganizeDatabaseReturnsOKOnMemberTimeout(t *testing.T) {
	var callOrder []bool
	app := &mockOrganizeDatabaseApp{
		organizeDBFunc: func(ctx context.Context, isMemberRoom bool) error {
			callOrder = append(callOrder, isMemberRoom)
			return context.DeadlineExceeded
		},
	}

	restore := stubOrganizeDatabaseDeps(t, app, nil, nil)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on handled timeout, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if len(callOrder) != 1 || callOrder[0] != true {
		t.Fatalf("expected processing to stop after member timeout, got %#v", callOrder)
	}
}

func TestOrganizeDatabaseReturnsOKOnGeneralTimeout(t *testing.T) {
	var callOrder []bool
	app := &mockOrganizeDatabaseApp{
		organizeDBFunc: func(ctx context.Context, isMemberRoom bool) error {
			callOrder = append(callOrder, isMemberRoom)
			if isMemberRoom {
				return nil
			}
			return context.DeadlineExceeded
		},
	}

	restore := stubOrganizeDatabaseDeps(t, app, nil, nil)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on handled timeout, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if len(callOrder) != 2 || callOrder[1] != false {
		t.Fatalf("expected member then general execution, got %#v", callOrder)
	}
}

func TestOrganizeDatabaseReturnsInitializationError(t *testing.T) {
	initErr := errors.New("credential failed")
	restore := stubOrganizeDatabaseDeps(t, nil, initErr, nil)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error after logging init failure, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
}

func TestOrganizeDatabaseReturnsWorkspaceAppInitializationError(t *testing.T) {
	initErr := errors.New("workspace init failed")
	restore := stubOrganizeDatabaseDeps(t, nil, nil, initErr)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error after logging workspace init failure, got %v", err)
	}
	if resp.Result != lambdautils.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
}

func stubOrganizeDatabaseDeps(t *testing.T, app organizeDatabaseApp, clientOptErr error, newAppErr error) func() {
	t.Helper()

	originalFirestoreClientOption := firestoreClientOption
	originalNewWorkspaceApp := newWorkspaceApp

	firestoreClientOption = func() (option.ClientOption, error) {
		return option.WithoutAuthentication(), clientOptErr
	}
	newWorkspaceApp = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (organizeDatabaseApp, error) {
		if newAppErr != nil {
			return nil, newAppErr
		}
		return app, nil
	}

	return func() {
		firestoreClientOption = originalFirestoreClientOption
		newWorkspaceApp = originalNewWorkspaceApp
	}
}
