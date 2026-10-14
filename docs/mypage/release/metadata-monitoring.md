# Metadata cleanup monitoring source

29日cleanupを定期呼出しへ接続するsourceと、再起動をまたぐ匿名の監視stateを用意した。実scheduler、Cloud runtime、IAM/credential、通知宛先は作成・設定していない。公開承認・実inventory・自然実行・delivery確認は[release gate](README.md)に残る。

## 接続するcallback

承認後のruntime側では、既に用意されたFirestore client、明示project、deadline対応の専用signal writerから組み立てる。以下はsource構成例であり、live実行コマンドではない。

```go
signals := &mypage.JSONMetadataCleanupHealthRecorder{Writer: deadlineWriter}
runner, err := mypage.NewFirestoreMetadataCleanupRunner(
    client, selectedProjectID, signals, time.Now, 100, 100,
)
// errを固定statusへ変換し、raw dependency errorを公開logへ出さない。
// cleanupのschedule callback: runner.Run(ctx)
// 独立watchdogのschedule callback: runner.CheckHealth(ctx)
```

`Run`は既存の10分budget・29日eligibility・revision guard・backlog検査を使う。監視stateの保存にはjob終了後の独立2秒budgetを使い、health readとsignal writeも各2秒に制限する。scheduler側はjob10分に後処理の余裕を加えたdeadlineを持ち、返るerrorを失敗扱いにする。キャンセルされたrunも失敗signalを試みる。`CheckHealth`はaccountをscan/clearせず、単独で呼べる。

cleanupは1日2回等、成功scan開始間隔が23時間未満になる配置が必要。独立watchdogは例として毎時に起動し、cleanupのtrigger欠落・停止も監視する。cleanupの終了時にhealthを確認するだけでは、次のcleanupが起動しない障害を検出できない。watchdog自身の未起動やsignal sink障害はscheduler/runtime側の失敗監視が必要で、このsourceだけで到達性を証明しない。

`JSONMetadataCleanupHealthRecorder`は、書込み中のdeadline/cancellationに対応する専用pipe/socket等のwriterを要求する。通常fileや無期限`io.Writer`は拒否する。`SetWriteDeadline`だけを形だけ実装したwriterを本番に使わない。writerを他の処理と共有しない。sink・routing・alert宛先は承認済みruntime inventoryから選び、例の値を実環境へ流用しない。

## 監視stateとsignal

既存client rulesでdefault denyとなるserver-only `mypage-maintenance/metadata-cleanup`へ、最新の匿名observationと`lastSuccessfulCoverage`をtransaction保存する。project単位のsingletonであり、user ID、metadata、credential、raw error、実行別履歴を保存しない。constructorがcleanupとmonitorの同じprojectを確認する。新しいquery indexは不要。

成功coverageは完了時刻ではなくscan開始時刻。古い実行の遅延到着・再送は新しい失敗を上書きせず、同じ開始時刻では失敗を優先する。monitoring stateが不正・破損している場合は成功値へresetせず拒否し、運用側の調査を必要とする。signal書込み失敗はrunのerrorへ返し、monitorへ保存できる場合はmonitoring失敗も残す。

health出力は`checkedAt`, `alert`, allowlist内の`reason`のみ。`healthy`は回復signal、`never_succeeded`/`run_failed`/`stale_heartbeat`/`retention_deadline`/`invalid_clock`/`incomplete_observation`/`monitoring_failed`はalert。retention違反は後の正常runで最新観測から消えるため、sink側のincident記録を後続のhealthyで削除しない。raw errorは内部でunwrap可能なまま返すが、公開logへ出さない。

## 合成検証と残るGate

mock/local pipeで未成功・23時間境界・再実行・取消・monitor/signal障害・不正観測の拒否を確認する。Firestore Emulatorでは実cleanupとmonitorを組み合わせ、公開metadataのみ除去、policy/access保持、同時transaction、adapter再生成後のcoverage、古い観測の再送、不正projectと破損stateの拒否を確認する。通常Go suiteと[既存Emulator script](../../../.github/scripts/run-firestore-integration-tests.sh)が検証入口。

実project・runtime・所有者、watchdog/cleanup schedule、監視sinkと通知delivery、失敗再試行、store/log/backup inventoryと保持は未確認。既存のlive gate・アプリ内Privacy受付flagを変更せず、実OAuth・実削除・deployment・scheduler activationを行わない。
