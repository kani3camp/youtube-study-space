# FSL formal specifications

YouTube Study Space の形式仕様を FSL で検証するための隔離されたツール層です。

## バージョン固定

`VERSION` に FSL の Release tag、`SHA256SUMS` に公式 Release asset の SHA-256 を固定しています。CI は `latest` を追従せず、固定バージョンの native `fslc` を取得してハッシュを検証します。

FSL の更新は通常の依存更新として、バージョン・checksum・全仕様の検証結果を同じ PR で確認してください。

## ローカル検証

全 FSL 仕様を検証する場合:

```bash
bash formal-spec/fsl/scripts/verify.sh
```

`SeatSession` の conformance vector 生成から Go 実装との照合まで含めた CI 相当の検証:

```bash
bash .github/scripts/run-formal-spec-tests.sh
```

人間向けの自己完結 HTML レポートを生成する場合は、spec basename を渡します。省略時は `seat_session` です。

```bash
bash formal-spec/fsl/scripts/report.sh seat_session
bash formal-spec/fsl/scripts/report.sh seat_allocation
bash formal-spec/fsl/scripts/report.sh chat_message_processing
```

対応している FSL 開発環境は Linux x86_64 / arm64 と macOS Apple Silicon です。`fslc` は `formal-spec/fsl/.tools/` にキャッシュされ、生成した conformance JSON / HTML は `formal-spec/fsl/generated/` に置かれ、どちらもコミットされません。

環境変数:

- `FSLC`: 既存の `fslc` 実行ファイルを明示する
- `FSL_TOOL_DIR`: ダウンロード先を変更する
- `FSL_VERIFY_DEPTH`: BMC の depth を変更する（既定 8）
- `FSL_CONFORMANCE_DEPTH`: SeatSession conformance vector の探索 depth を変更する（既定 4）
- `FSL_REPORT_DEPTH`: HTML レポートの検証 depth を変更する（既定 8）
- `FSL_GENERATED_DIR`: conformance JSON の生成先を変更する

## Requirement traceability

- `SeatSession`: `REQ-SEAT-*`
- `SeatAllocation`: `REQ-ALLOC-*`
- `ChatMessageProcessing`: `REQ-CHAT-*`

これらを FSL の正準な `@requirement(id, text)` metadata として対象 action / invariant に付与します。requirement metadata は検証そのものの代替ではありません。契約の意味は action / invariant の式で検証し、ID と説明はレビュー時のトレーサビリティとして使います。

## CI gate

PR の `Formal Spec (FSL)` job では `specs/*.fsl` 全体に対して `check` と bounded verification を実行します。

```text
all specs: fslc check
all specs: fslc verify --depth 8 --vacuity error
SeatSession: fslc conformance --depth 4
SeatSession: Go conformance adapter (build tag: formalspec)
```

`SeatSession` は実 Go adapter まで接続済みです。`SeatAllocation` は高位の割当安全モデル、`ChatMessageProcessing` はP1移行先の目標契約として検証します。後者2つは現行実装との conformance 未接続であり、FSL検証が通っても「現在のGo/Firestore実装が契約を満たす」とは扱いません。

`ChatMessageProcessing` は、durable ingest と checkpoint、message processor のlease/retry/dead-letter、domain effect と reply outbox intent の atomicity を対象にします。外部 YouTube/Discord 配送の exactly-once は保証対象外です。

Go adapter は FSL の `conformance.v1` JSON を読み、各 reachable state / action instance について `SeatDoc` の実装結果を照合します。`requires_failed` の vector では Go 側も error を返し、`SeatDoc` が変更されないことまで確認します。仕様で新しい outcome が現れた場合は自動で skip せず、分類を要求して失敗します。

## Review report

`Formal Spec Report` workflow は形式仕様が変わった PR で全 `specs/*.fsl` に対して `fslc html --depth 8` を実行し、自己完結 HTML 群を GitHub Actions artifact `fsl-spec-reports` として 14 日間保持します。main formal gate と分離することで、検証・conformance の gate にレポート生成コストを重複させません。

レポートは state / action / property、requirement metadata、検証結果、witness などを CLI なしでレビューするための補助資料です。どれか1つでも HTML 生成に失敗した場合は `Formal Spec Report` workflow 自体が失敗します。

Linux バイナリは Ubuntu 24.04 ABI を前提としているため、専用 CI job は `ubuntu-24.04` を使います。

## 境界

FSL は本番依存ではありません。Go / Firestore / Lambda / ECS から FSL を呼び出さず、形式仕様の導入・撤去が本番成果物に波及しない状態を維持します。Go conformance adapter も `formalspec` build tag のテスト専用コードであり、通常ビルドには含まれません。
