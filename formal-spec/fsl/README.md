# FSL formal specifications

YouTube Study Space の形式仕様を FSL で検証するための隔離されたツール層です。

## バージョン固定

`VERSION` に FSL の Release tag、`SHA256SUMS` に公式 Release asset の SHA-256 を固定しています。CI は `latest` を追従せず、固定バージョンの native `fslc` を取得してハッシュを検証します。

FSL の更新は通常の依存更新として、バージョン・checksum・全仕様の検証結果を同じ PR で確認してください。

## ローカル検証

FSL 仕様だけを検証する場合:

```bash
bash formal-spec/fsl/scripts/verify.sh
```

`SeatSession` の conformance vector 生成から Go 実装との照合まで含めた CI 相当の検証:

```bash
bash .github/scripts/run-formal-spec-tests.sh
```

人間向けの自己完結 HTML レポートだけを生成する場合:

```bash
bash formal-spec/fsl/scripts/report.sh
```

対応している FSL 開発環境は Linux x86_64 / arm64 と macOS Apple Silicon です。`fslc` は `formal-spec/fsl/.tools/` にキャッシュされ、生成した conformance JSON / HTML は `formal-spec/fsl/generated/` に置かれ、どちらもコミットされません。

環境変数:

- `FSLC`: 既存の `fslc` 実行ファイルを明示する
- `FSL_TOOL_DIR`: ダウンロード先を変更する
- `FSL_VERIFY_DEPTH`: BMC の depth を変更する（既定 8）
- `FSL_CONFORMANCE_DEPTH`: conformance vector の探索 depth を変更する（既定 4）
- `FSL_REPORT_DEPTH`: HTML レポートの検証 depth を変更する（既定 8）
- `FSL_GENERATED_DIR`: conformance JSON / HTML の生成先を変更する

## Requirement traceability

`SeatSession` の既存契約 ID `REQ-SEAT-001` 〜 `REQ-SEAT-005` はコメントではなく、FSL の正準な `@requirement(id, text)` metadata として対象 action / invariant に付与します。これにより verifier の診断や HTML レポートから、どの宣言がどの契約を担うか追跡できます。

requirement metadata は検証そのものの代替ではありません。契約の意味は action / invariant の式で検証し、ID と説明はレビュー時のトレーサビリティとして使います。

## CI gate

PR の形式仕様 job では以下を実行します。

```text
fslc check
fslc verify --depth 8 --vacuity error
fslc conformance seat_session.fsl --depth 4
Go conformance adapter (build tag: formalspec)
fslc html --depth 8
```

Go adapter は FSL の `conformance.v1` JSON を読み、各 reachable state / action instance について `SeatDoc` の実装結果を照合します。`requires_failed` の vector では Go 側も error を返し、`SeatDoc` が変更されないことまで確認します。仕様で新しい outcome が現れた場合は自動で skip せず、分類を要求して失敗します。

`Formal Spec Report` workflow は形式仕様が変わった PR で `seat_session.html` を生成し、GitHub Actions artifact `seat-session-fsl-report` として 14 日間保持します。レポートは自己完結 HTML で、state / action / property、requirement metadata、検証結果、witness などを CLI なしでレビューするための補助資料です。

Linux バイナリは Ubuntu 24.04 ABI を前提としているため、専用 CI job は `ubuntu-24.04` を使います。

## 境界

FSL は本番依存ではありません。Go / Firestore / Lambda / ECS から FSL を呼び出さず、形式仕様の導入・撤去が本番成果物に波及しない状態を維持します。Go conformance adapter も `formalspec` build tag のテスト専用コードであり、通常ビルドには含まれません。
