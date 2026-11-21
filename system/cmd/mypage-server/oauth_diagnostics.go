package main

import (
	"context"
	"log"

	"app.modules/core/mypage"
)

// DEVの診断専用。リクエストは固定値だけをキューへ渡し、ログI/Oを待たない。
// キューが満杯のときや終了時には診断を破棄できるため、完全な監査記録ではない。
func oauthFailureDiagnostics(ctx context.Context, logger *log.Logger) chan<- mypage.OAuthFailureObservation {
	queue := make(chan mypage.OAuthFailureObservation, 32)
	go func() {
		// 診断先のpanicもサービスへ伝播させず、値は出力しない。
		defer func() {
			if recover() != nil {
				return
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case observation := <-queue:
				logOAuthFailure(logger, observation)
			}
		}
	}()
	return queue
}

func logOAuthFailure(logger *log.Logger, observation mypage.OAuthFailureObservation) {
	logger.Printf("mypage_oauth_failure stage=%s code=%s", observation.Stage(), observation.ErrorCode())
}
