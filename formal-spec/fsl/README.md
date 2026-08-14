# FSL formal specifications

YouTube Study Space の形式仕様を FSL で検証するための隔離されたツール層です。

## バージョン固定

`VERSION` に FSL の Release tag、`SHA256SUMS` に公式 Release asset の SHA-256 を固定しています。CI は `latest` を追従せず、固定バージョンの native `fslc` を取得してハッシュを検証します。

FSL の更新は通常の依存更新として、バージョン・checksum・全仕様の検証結果を同じ PR で確認してください。

## ローカル検証

```bash
bash formal-spec/fsl/scripts/verify.sh
```

対応している開発環境は Linux x86_64 / arm64 と macOS Apple Silicon です。`fslc` は `formal-spec/fsl/.tools/` にキャッシュされ、コミットされません。

環境変数:

- `FSLC`: 既存の `fslc` 実行ファイルを明示する
- `FSL_TOOL_DIR`: ダウンロード先を変更する
- `FSL_VERIFY_DEPTH`: BMC の depth を変更する（既定 8）

## CI gate

PR では各 `.fsl` に対して以下を実行します。

```text
fslc check
fslc verify --depth 8 --vacuity error
```

Linux バイナリは Ubuntu 24.04 ABI を前提としているため、専用 CI job は `ubuntu-24.04` を使います。

## 境界

FSL は本番依存ではありません。Go / Firestore / Lambda / ECS から FSL を呼び出さず、形式仕様の導入・撤去が本番成果物に波及しない状態を維持します。
