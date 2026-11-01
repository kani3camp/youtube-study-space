package mypage

import "errors"

// OAuthFailureObservationは識別子・依存先エラー・リクエストを保持しない。
// 出力には必ずStage/ErrorCodeの固定allowlist表現だけを使う。
type OAuthFailureObservation struct {
	stage oauthFailureStage
	code  string
}

type oauthFailureStage uint8

const (
	oauthStageUnknown oauthFailureStage = iota
	oauthStagePurposeRead
	oauthStageMaintenance
	oauthStageCallbackInput
	oauthStageClaim
	oauthStageDenied
	oauthStageResolve
	oauthStageCode
	oauthStageExchange
	oauthStageScope
	oauthStageToken
	oauthStageChannelRequest
	oauthStageChannelTransport
	oauthStageChannelStatus
	oauthStageChannelDecode
	oauthStageChannelSelection
	oauthStageFail
	oauthStageConfirmation
	oauthStageVerify
)

func (o OAuthFailureObservation) Stage() string {
	switch o.stage {
	case oauthStagePurposeRead:
		return "purpose_read"
	case oauthStageMaintenance:
		return "maintenance_check"
	case oauthStageCallbackInput:
		return "callback_input"
	case oauthStageClaim:
		return "transaction_claim"
	case oauthStageDenied:
		return "provider_denied"
	case oauthStageResolve:
		return "provider_resolve"
	case oauthStageCode:
		return "code_required"
	case oauthStageExchange:
		return "token_exchange"
	case oauthStageScope:
		return "scope_validation"
	case oauthStageToken:
		return "token_validation"
	case oauthStageChannelRequest:
		return "channel_request"
	case oauthStageChannelTransport:
		return "channel_transport"
	case oauthStageChannelStatus:
		return "channel_status"
	case oauthStageChannelDecode:
		return "channel_decode"
	case oauthStageChannelSelection:
		return "channel_selection"
	case oauthStageFail:
		return "transaction_fail"
	case oauthStageConfirmation:
		return "confirmation_generate"
	case oauthStageVerify:
		return "transaction_verify"
	default:
		return "unknown"
	}
}

func (o OAuthFailureObservation) ErrorCode() string {
	return errorCode(&APIError{Code: o.code})
}

// stageは内部の列挙値だけ。プロバイダーでは生エラーを破棄した後に付ける。
// Error/Unwrapは既存の公開コードとerrors.Is/Asの契約を維持する。
type oauthStageError struct {
	stage oauthFailureStage
	err   error
}

func (e *oauthStageError) Error() string { return e.err.Error() }
func (e *oauthStageError) Unwrap() error { return e.err }

func oauthFailure(stage oauthFailureStage, err error) error {
	var marked *oauthStageError
	if errors.As(err, &marked) {
		return err
	}
	return &oauthStageError{stage: stage, err: err}
}

func (s *AuthService) observeOAuthFailure(err error) {
	if err == nil || s.OAuthFailures == nil {
		return
	}
	stage := oauthStageUnknown
	var marked *oauthStageError
	if errors.As(err, &marked) {
		stage = marked.stage
	}
	observation := OAuthFailureObservation{stage: stage, code: errorCode(err)}
	// 終了済み受信先も認証結果に影響させない。panicの内容は保持・出力しない。
	defer func() {
		if recover() != nil {
			return
		}
	}()
	// 診断先の遅延・不在・満杯は捨てる。認証中にログI/Oやgoroutine生成をしない。
	select {
	case s.OAuthFailures <- observation:
	default:
	}
}
