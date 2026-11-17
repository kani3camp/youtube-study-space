package mypage

// 利用者が同意するプライバシーポリシー・利用規約の版。本文と同じくソースで管理し、
// 環境変数では変えない。WebAccount には同意時の版を記録し、現在の版と異なる場合は再同意を求める。
// 本文を改訂して再同意を求めるときに上げ、mypage/src/public-policy-config.ts の
// POLICY_VERSIONS も同じ値にする（mypage/src/public-policy-config.test.ts で照合する）。
const (
	CurrentPrivacyPolicyVersion = "v1"
	CurrentTermsVersion         = "v1"
)

// CurrentPolicy は公開中の本文の版を返す。
func CurrentPolicy() Policy {
	return Policy{Privacy: CurrentPrivacyPolicyVersion, Terms: CurrentTermsVersion}
}

// ConsentVersionsHeader はAPIが受け付ける同意版を示す応答ヘッダー。公開本文の版だけを含む。
const ConsentVersionsHeader = "X-MyPage-Consent-Versions"

func (p Policy) header() string {
	return "privacy=" + p.Privacy + ", terms=" + p.Terms
}
