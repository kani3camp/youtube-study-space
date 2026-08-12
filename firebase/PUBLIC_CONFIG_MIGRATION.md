# Firestore public monitor config migration

`public-config/monitor`はyoutube-monitorへ公開する唯一のconfig documentです。次の5フィールドだけを保持します。

- `max-seats`
- `member-max-seats`
- `min-vacancy-rate`
- `youtube-membership-enabled`
- `fixed-max-seats-enabled`

`desired-max-seats`、`desired-member-max-seats`などのbackend専用設定は`config/constants`に残します。

## PR Bの本番反映手順

本番には`config/constants`しか存在しない前提で、次の順序を守ります。CodexやCIから本番dataは変更しません。

1. PR Aを反映し、Rules専用CIがgreenであることを確認します。
2. PR Bの`firebase/firestore.rules`だけを先にdeployし、`public-config/monitor`のanonymous read／client write denyを有効にします。

   ```bash
   firebase deploy --config firebase/firebase.json --project <production-project-id> --only firestore:rules
   ```

3. production用のGoogle Cloud認証情報とproject IDを確認し、まずpreviewを実行します。この操作はread onlyです。

   ```bash
   cd system
   go run ./cmd/bootstrap-public-monitor-config --project-id <production-project-id>
   ```

   ADCを使わない場合だけ`--credentials-file <path>`を追加します。

4. previewのproject IDと5値を既存`config/constants`と照合後、`--apply`を付けて再実行し、表示されたproject IDを入力します。

   ```bash
   go run ./cmd/bootstrap-public-monitor-config --project-id <production-project-id> --apply
   ```

   このcommandはFirestoreのcreateだけを使うため、既存の`public-config/monitor`を上書きしません。

5. Firebase Consoleまたはserver clientで、target documentが上記5フィールドだけを持つことを確認します。
6. PR Bのbackendをdeployします。通常の最大席数更新は以後`public-config/monitor`だけを書き換えます。`config/constants`側の旧5フィールドはmigration fallback用に残りますが、新正本としては読みません。
7. PR Cのyoutube-monitorをdeployし、horizontal／vertical双方の表示とrealtime updateを確認します。
8. 旧monitor buildが残っていないことを確認してからPR DのRulesをdeployし、`config/*`を完全非公開化します。

## Rollback

PR BまたはPR Cまでのrollbackでは`config/constants`のpublic readを維持しているため、旧monitor buildへ戻せます。`public-config/monitor`は削除せず、新backendをrollbackした場合は必要に応じて5値を`config/constants`へ手動で戻します。PR Dをdeployした後に旧monitorへrollbackする場合は、先にPR DのRulesをrollbackしないと旧buildがconfigを読めません。
