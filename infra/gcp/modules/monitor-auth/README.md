# モニターのメール・パスワード認証

既存Firebase Authenticationのメール・パスワード認証だけをTerraformで管理します。メール・パスワードでの公開登録と既存アカウントへの方式追加を許可し、大量登録等のリスクを受け入れる方針です。`beforeCreate`・`beforeSignIn`による登録・ログイン制限は設けません。テナントや別プロジェクトへの分離もしません。

モニターのデータ読み取りには、Firebaseログインに加え、Firestoreの `_monitor-readers/{uid}` が `enabled: true` であることを引き続き要求します。アカウントを登録しただけではモニターを閲覧できません。運用者のアカウント作成・許可リストの準備は[モニター手順](../../../../youtube-monitor/README.md)を参照してください。

## 管理範囲

- メール・パスワード認証の有効化とパスワード必須設定。
- 既存Auth設定の限定importと、Auth全体の削除防止。
- 既存のユーザー・他のIdP・許可ドメイン・MFA・App Check・トリガー等を維持。

環境rootの `monitor_email_password_enabled` は既定で `false` です。DEVは明示的に有効、本番は無効のままです。Firebase CLIから同じAuth設定を配備しません。登録制限用の関数・ソースバケット・Artifact Registry・専用実行／ビルドSAは不要です。

## 適用済み登録制限の撤去

旧DEV Stateを持つ環境では、最終コードへ直接飛ばず、[PR #1467](https://github.com/kani3camp/youtube-study-space/pull/1467)の記録と固定コミットを使って段階適用します。各段階で通常Plan／Apply、独立Plan一致、事後full-root no-opを確認し、無関係な差分があれば停止します。

1. `070b79a0`：Authの `beforeCreate` だけを解除します。関数は残し、公開登録と既存アカウントのログイン、他のAuth設定が維持されることを確認します。
2. `d1f59c7f`：関数とZIPの削除ポリシーを変更し、専用ソースバケットだけに一時的なオブジェクト削除権限を設定します。資源はまだ削除しません。
3. `a91e69b5`：専用関数・公開invoker・専用リポジトリのビルドgrant・ZIPの4資源だけを削除します。
4. `f9e30650`：バケットが空であることを確認し、専用バケット・一時grant・専用Artifact Registryを削除します。`moved` でAuthの管理アドレスだけを移し、Authの削除・再作成は行いません。
5. 実体の不在とStateの整合を確認してから、専用SAの他用途がないことを確認し、登録制限専用のIAM grant・SAと、この関数だけの管理バケット内ソースコピーを撤去します。Auth設定の参照／更新権限は、メール認証を管理するため維持します。共有バケット・API・Identity Platform・既存ユーザーは削除しません。

段階適用後の事後検証では、IAM変更に伴うバケットの `updated` やリポジトリの `update_time` が観測差分として検知されることがあります。検証ゲートを緩めず、対象コミットと同じHCLで、資源変更0件・観測項目だけの差分であることを非公開のnative refresh-only Planで確認してください。StateのVersion・lineage・native lockと他資源の値・依存関係を前後比較し、通常のfull-root Plan／Applyで整合を再確認します。異なる段階のHCLや古いStateを復旧に使用しないでください。実施結果はPRに記録します。

ソースバケットのsoft delete等、クラウド側の既存保管期間は変更しません。削除済み資源の回復可能なコピーがその期間だけ残る場合があります。撤去後は `normal-operations.json` の一回限りの削除指定を消し、同じ最終コードでfull-root no-opを再確認します。State削除・巻き戻し・`terraform state rm`・target applyで撤去を代用しません。

## 検証

```sh
python3 -B infra/gcp/tests/test_monitor_auth.py
python3 -B infra/gcp/scripts/test_monitor_auth_terraform.py
python3 -B infra/gcp/scripts/inspect_dev_monitor_auth.py --target dev
```

構成確認は非秘密の末端フィールドだけを取得します。Auth Config全体や `signIn` 全体にはパスワードハッシュ用の秘密パラメーターが含まれるため、生の応答・State・PlanをログやPRへ出しません。通常のDEV／本番運用は[GCP Terraform手順](../../README.md)に従います。
