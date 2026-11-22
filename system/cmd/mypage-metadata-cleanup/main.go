// このCloud Run JobはMyPageの公開メタデータ削除（-mode=run）と独立監視（-mode=health）を実行する。
// 削除と監視は別々に起動する前提で、起動間隔・再試行・失敗通知は呼び出し側（スケジューラー等）で設定する。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/oauth2/google"

	"app.modules/core/mypage"
)

const (
	modeRun    = "run"
	modeHealth = "health"
	// 削除処理本体の予算（MetadataCleanupBudget）に、監視状態の保存・健全性シグナル・ACK の後処理分を加える。
	runTimeout    = mypage.MetadataCleanupBudget + time.Minute
	healthTimeout = 30 * time.Second
	// 1回の実行で最大 100 件 × 100 ページを処理する。残件があれば成功扱いにせず監視で検知する。
	cleanupBatchSize = 100
	cleanupMaxPages  = 100
)

var errInvalidArguments = errors.New("mode must be exactly one of -mode=run or -mode=health")

type target struct {
	project, location, job string
}

// 環境ごとの実行先は固定し、環境変数の値だけで任意のプロジェクトへ向けられないようにする。
var targets = map[string]struct{ project, location string }{
	"development": {"test-youtube-study-space", "asia-southeast2"},
	"production":  {"youtube-study-space", "asia-northeast2"},
}

func parseMode(args []string) (string, error) {
	flags := flag.NewFlagSet("mypage-metadata-cleanup", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode, seen := "", false
	flags.Func("mode", "run or health", func(value string) error {
		if seen {
			return errInvalidArguments
		}
		mode, seen = value, true
		return nil
	})
	if flags.Parse(args) != nil || flags.NArg() != 0 || (mode != modeRun && mode != modeHealth) {
		return "", errInvalidArguments
	}
	return mode, nil
}

func resolveTarget(getenv func(string) string) (target, error) {
	fixed, ok := targets[getenv("MYPAGE_ENVIRONMENT")]
	if !ok || fixed.project != getenv("GOOGLE_CLOUD_PROJECT") {
		return target{}, errors.New("invalid environment or project")
	}
	// Cloud Run Job が設定する実行中のジョブ名を、ログのリソースラベルに使う。
	job := getenv("CLOUD_RUN_JOB")
	if job == "" {
		return target{}, errors.New("missing Cloud Run Job name")
	}
	return target{project: fixed.project, location: fixed.location, job: job}, nil
}

func timeoutFor(mode string) time.Duration {
	if mode == modeRun {
		return runTimeout
	}
	return healthTimeout
}

type cleanupRunner interface {
	Run(context.Context) (mypage.MetadataCleanupObservation, error)
	CheckHealth(context.Context) (mypage.MetadataCleanupHealthObservation, error)
}

// 標準出力には匿名の件数・時刻・固定の段階名/理由だけを書く。依存先の生エラーは出力しない。
type runResult struct {
	Mode string `json:"mode"`
	mypage.MetadataCleanupObservation
}

type healthResult struct {
	Mode string `json:"mode"`
	mypage.MetadataCleanupHealthObservation
}

func execute(ctx context.Context, mode string, runner cleanupRunner, output io.Writer) error {
	var result any
	var err error
	switch mode {
	case modeRun:
		var observation mypage.MetadataCleanupObservation
		observation, err = runner.Run(ctx)
		result = runResult{Mode: modeRun, MetadataCleanupObservation: observation}
	case modeHealth:
		var signal mypage.MetadataCleanupHealthObservation
		signal, err = runner.CheckHealth(ctx)
		result = healthResult{Mode: modeHealth, MetadataCleanupHealthObservation: signal}
	default:
		return errInvalidArguments
	}
	if encodeErr := json.NewEncoder(output).Encode(result); encodeErr != nil {
		err = errors.Join(err, fmt.Errorf("write result: %w", encodeErr))
	}
	return err
}

// main は失敗時に固定の状態文字列だけを標準エラーへ出し、生エラー（ID や依存先の応答を含みうる）は公開ログに出さない。
func main() {
	if status := runMain(os.Args[1:], os.Getenv, os.Stdout); status != "" {
		fmt.Fprintln(os.Stderr, status)
		os.Exit(1)
	}
}

// runMain は引数と実行先を検査してから認証情報・Firestore クライアントを作る。失敗時は固定の状態文字列を返す。
func runMain(args []string, getenv func(string) string, output io.Writer) string {
	mode, err := parseMode(args)
	if err != nil {
		return err.Error()
	}
	selected, err := resolveTarget(getenv)
	if err != nil {
		return err.Error()
	}
	signals := &mypage.CloudLoggingMetadataCleanupHealthRecorder{ProjectID: selected.project, Location: selected.location, JobName: selected.job}
	if signals.ValidateResource() != nil {
		return "invalid Cloud Run Job name"
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutFor(mode))
	defer cancel()
	// トークンソースはプロセス全体で使うため、実行期限付きの ctx には結び付けない。取得の待機は記録時の ctx で打ち切る。
	signals.Tokens, err = google.DefaultTokenSource(context.Background(), mypage.CloudLoggingWriteScope)
	if err != nil {
		return "metadata cleanup: credentials unavailable"
	}
	client, err := firestore.NewClient(ctx, selected.project)
	if err != nil {
		return "metadata cleanup: connect Firestore failed"
	}
	runner, err := mypage.NewFirestoreMetadataCleanupRunner(client, selected.project, signals, time.Now, cleanupBatchSize, cleanupMaxPages)
	if err == nil {
		err = execute(ctx, mode, runner, output)
	}
	closeErr := client.Close()
	switch {
	case err != nil:
		return "metadata cleanup: " + mode + " failed"
	case closeErr != nil:
		return "metadata cleanup: close Firestore failed"
	}
	return ""
}
