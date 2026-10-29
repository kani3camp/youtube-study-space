package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

var (
	errIntake     = errors.New("human Privacy intake unavailable")
	intakeChannel = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
	intakeActor   = regexp.MustCompile(`^[A-Za-z0-9_-]{3,128}$`)
)

// 有人窓口のやり取りは窓口に留め、本文や連絡先をコピーしない。
// 対象指定は申告に過ぎず、既存の申請に結び付けた新たなOAuth認証を待つ。
type humanRequest struct {
	RequestID        string
	Channel          string
	AcceptedAt       time.Time
	ExpectedRevision time.Time
}

type intakeIntent struct {
	Action               string
	Environment, Project string
	Purpose              mypage.SupportPurpose
	Request              humanRequest
}

type intakeGrant struct {
	Actor  string
	Intent intakeIntent
}

type supportCreator interface {
	Create(context.Context, string, string, mypage.SupportPurpose, time.Time, time.Time) (string, string, error)
}

type intakeReceipt struct {
	RequestRef, Challenge string
	Revision              time.Time
	DeleteBy              *time.Time
}

type intakeRuntime struct {
	close                func()
	environment, project string
	creator              supportCreator
	recovery             supportRecovery
	// 申請ID・対象チャンネル・サポートチャレンジ・Firebase本人情報・ADCだけでは運用権限にならない。
	// 実環境はliveIntakeFactoryがowner-controlled GCP IAMで組み立てる。
	authorize func(context.Context, intakeIntent) (intakeGrant, error)
	// 非公開の受け渡しは標準出力や監査とは分離し、依頼された操作の完了証拠やACKとして扱わない。
	// 実際の配送機能は含めない。
	deliver func(context.Context, intakeIntent, intakeReceipt) error
	clock   func() time.Time
}

type intakeFactory func(context.Context, proposal) (*intakeRuntime, error)

func intakePurpose(p mypage.SupportPurpose) bool {
	return p == mypage.SupportDelete || p == mypage.SupportRevoke || p == mypage.SupportDisclosure
}

func readHumanRequest(path, action string) (humanRequest, error) {
	data, err := privateFile(path, 1024)
	if err != nil {
		return humanRequest{}, errIntake
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return humanRequest{}, errIntake
	}
	values := make(map[string]string, 3)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || (key != "requestId" && key != "targetChannel" && key != "acceptedAt" && (key != "expectedReceiptVersion" || action != "recover")) {
			return humanRequest{}, errIntake
		}
		if _, duplicate := values[key]; duplicate {
			return humanRequest{}, errIntake
		}
		var value string
		if decoder.Decode(&value) != nil || value == "" {
			return humanRequest{}, errIntake
		}
		values[key] = value
	}
	expectedFields := 3
	if action == "recover" {
		expectedFields = 4
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') || len(values) != expectedFields {
		return humanRequest{}, errIntake
	}
	if _, err := decoder.Token(); err != io.EOF {
		return humanRequest{}, errIntake
	}
	at, err := time.Parse(time.RFC3339Nano, values["acceptedAt"])
	if err != nil || at.IsZero() || at.After(time.Now()) || values["acceptedAt"] != at.UTC().Format(time.RFC3339Nano) || !at.Equal(at.Truncate(time.Microsecond)) || !opaque.MatchString(values["requestId"]) || !intakeChannel.MatchString(values["targetChannel"]) {
		return humanRequest{}, errIntake
	}
	input := humanRequest{RequestID: values["requestId"], Channel: values["targetChannel"], AcceptedAt: at}
	if action == "recover" {
		version, err := time.Parse(time.RFC3339Nano, values["expectedReceiptVersion"])
		if err != nil || version.IsZero() || values["expectedReceiptVersion"] != version.UTC().Format(time.RFC3339Nano) {
			return humanRequest{}, errIntake
		}
		input.ExpectedRevision = version
	}
	return input, nil
}

// newFirestoreIntakeRuntimeは受付作成と保護付き復旧を組み立てる。
// 承認済みクライアント・本人情報・非公開配送の管理は呼び出し元が担う。
// 認証情報の探索・認証・通信は行わない。
func newFirestoreIntakeRuntime(p proposal, client *firestore.Client, authorize func(context.Context, intakeIntent) (intakeGrant, error), deliver func(context.Context, intakeIntent, intakeReceipt) error, clock func() time.Time) (*intakeRuntime, error) {
	if client == nil || !project.MatchString(p.Project) || (p.Environment != "development" && p.Environment != "production") || authorize == nil || deliver == nil || clock == nil ||
		!strings.HasPrefix(client.Collection("support-requests").Path, "projects/"+p.Project+"/databases/(default)/documents/") {
		return nil, errIntake
	}
	store := &mypage.FirestoreSupportStore{Client: client, Environment: p.Environment}
	return &intakeRuntime{environment: p.Environment, project: p.Project, creator: store, recovery: store, authorize: authorize, deliver: deliver, clock: clock}, nil
}

func runHumanIntake(ctx context.Context, p proposal, input humanRequest, factory intakeFactory) string {
	if ctx == nil || ctx.Err() != nil || factory == nil {
		return "TRUSTED_INTAKE_UNAVAILABLE"
	}
	runtime, err := factory(ctx, p)
	if err != nil || ctx.Err() != nil || runtime == nil || (p.Action == "create" && runtime.creator == nil) || (p.Action != "create" && runtime.recovery == nil) || runtime.authorize == nil || runtime.deliver == nil || runtime.clock == nil || runtime.environment != p.Environment || runtime.project != p.Project {
		return "TRUSTED_INTAKE_UNAVAILABLE"
	}
	if runtime.close != nil {
		defer runtime.close()
	}
	now := runtime.clock().UTC()
	if now.IsZero() || input.AcceptedAt.IsZero() || input.AcceptedAt.After(now) {
		return "INVALID_INPUT"
	}
	intent := intakeIntent{Action: p.Action, Environment: p.Environment, Project: p.Project, Purpose: p.Purpose, Request: input}
	grant, err := runtime.authorize(ctx, intent)
	if err != nil || ctx.Err() != nil || grant.Intent != intent || !intakeActor.MatchString(grant.Actor) {
		return "TRUSTED_INTAKE_DENIED"
	}
	// 権限確認後に信頼済み時刻を読み直し、元の受付時刻を維持する。
	// Createは1回だけ呼び、タイムアウトやACK消失でSLAをリセットしない。
	now = runtime.clock().UTC()
	if now.IsZero() || input.AcceptedAt.After(now) {
		return "INVALID_INPUT"
	}
	if p.Action != "create" {
		return runHumanRecovery(ctx, p, input, runtime, intent, now)
	}
	ref, challenge, err := runtime.creator.Create(ctx, input.RequestID, input.Channel, p.Purpose, input.AcceptedAt, now)
	if err != nil || ctx.Err() != nil || !opaque.MatchString(ref) || !opaque.MatchString(challenge) {
		return "INTAKE_RESULT_UNCERTAIN"
	}
	if runtime.deliver(ctx, intent, intakeReceipt{RequestRef: ref, Challenge: challenge}) != nil || ctx.Err() != nil {
		return "INTAKE_HANDOFF_PENDING"
	}
	return "MOCK_AWAITING_PROOF"
}
