package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/api/option"

	"app.modules/internal/awsruntime"
)

func TestOrganizeDatabaseFailuresDoNotLogPrivateDependencyDetails(t *testing.T) {
	const channelID = "PRIVATE_CHANNEL_ID_546"
	const content = "PRIVATE_WORK_CONTENT_966"
	const credential = "PRIVATE_CREDENTIAL_756"
	cause := errors.New("synthetic dependency sentinel")
	privateErr := fmt.Errorf("channel=%s content=%s credential=%s: %w", channelID, content, credential, cause)
	for _, tc := range []struct {
		name      string
		stage     string
		wantClass string
	}{
		{name: "firestore option", stage: "option", wantClass: "firestore_option_failed"},
		{name: "workspace init", stage: "init", wantClass: "workspace_init_failed"},
		{name: "organize failure", stage: "room", wantClass: "organize_failed"},
		{name: "organize timeout", stage: "timeout", wantClass: "deadline_exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })

			var optionErr, initErr error
			if tc.stage == "option" {
				optionErr = privateErr
			}
			if tc.stage == "init" {
				initErr = privateErr
			}
			app := &mockOrganizeDatabaseApp{organizeDBFunc: func(_ context.Context, isMemberRoom bool) error {
				if !isMemberRoom {
					return nil
				}
				if tc.stage == "timeout" {
					return fmt.Errorf("channel=%s credential=%s: %w", channelID, credential, context.DeadlineExceeded)
				}
				return privateErr
			}}
			restore := stubOrganizeDatabaseDeps(t, app, optionErr, initErr)
			t.Cleanup(restore)

			resp, err := OrganizeDatabase(context.Background())
			if err != nil || resp.Result != awsruntime.OK {
				t.Fatalf("handler response changed: response=%#v err=%v", resp, err)
			}
			if tc.stage == "room" {
				if len(app.messageToOwnerCalls) != 1 || !strings.Contains(app.messageToOwnerCalls[0], content) {
					t.Fatalf("existing owner delivery changed: %#v", app.messageToOwnerCalls)
				}
			} else if len(app.messageToOwnerCalls) != 0 {
				t.Fatalf("unexpected owner delivery: %#v", app.messageToOwnerCalls)
			}
			if !strings.Contains(logs.String(), `"error_class":"`+tc.wantClass+`"`) {
				t.Fatalf("missing safe failure class: %s", logs.String())
			}
			for _, private := range []string{channelID, content, credential} {
				if strings.Contains(logs.String(), private) {
					t.Fatalf("process log exposed dependency payload: %s", logs.String())
				}
			}
		})
	}
}

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
	if resp.Result != awsruntime.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if !app.closed {
		t.Fatal("expected firestore client to be closed")
	}
}

func TestOrganizeDatabaseLogsRoomFailuresAndReturnsOKAfterBothRoomsRun(t *testing.T) {
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
	if resp.Result != awsruntime.OK {
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
	if resp.Result != awsruntime.OK {
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
	if resp.Result != awsruntime.OK {
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
	if resp.Result != awsruntime.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
	if len(callOrder) != 2 || callOrder[1] != false {
		t.Fatalf("expected member then general execution, got %#v", callOrder)
	}
}

func TestOrganizeDatabaseLogsInitializationFailureAndReturnsOK(t *testing.T) {
	initErr := errors.New("credential failed")
	restore := stubOrganizeDatabaseDeps(t, nil, initErr, nil)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error after logging init failure, got %v", err)
	}
	if resp.Result != awsruntime.OK {
		t.Fatalf("expected ok result, got %#v", resp)
	}
}

func TestOrganizeDatabaseLogsWorkspaceAppInitializationFailureAndReturnsOK(t *testing.T) {
	initErr := errors.New("workspace init failed")
	restore := stubOrganizeDatabaseDeps(t, nil, nil, initErr)
	defer restore()

	resp, err := OrganizeDatabase(context.Background())
	if err != nil {
		t.Fatalf("expected nil error after logging workspace init failure, got %v", err)
	}
	if resp.Result != awsruntime.OK {
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
