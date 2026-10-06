# Retained user-activity-history adoption

Existing BigQuery `firestore_export.user-activity-history` のdevelopment adoption moduleです。

## Canonical schema

Terraform ownershipへ入れるschemaは次の8 fieldだけです。

- `seat_id`
- `taken_at`
- `user_id`
- `activity_type`
- `is_member_seat`
- `__key__`
- `__error__`
- `__has_error__`

legacy `timestamp` はcanonicalではなく、module validationが存在を許しません。

## Activation boundary

root definitionはdefault-offです。

次を満たす前に `manage_user_activity_history=true` にしないでください。

1. #1192のretained schema guardが対象development runtimeへ反映済み
2. aggregate-only auditでlegacy値の意味を確認
3. external BI / cross-project / legacy SQL / wildcard consumer確認
4. rollback / short-lived recoveryを確定
5. schema repairをTerraform importとは別mutationとして完了
6. fresh table metadataでcanonical 8 fieldsと実field orderを再取得

`user_activity_history_field_order` はfresh remote metadataから与え、推測やproduction順序のコピーをしません。

## Import contract

schema repair完了後の別waveで:

- exact table import1
- existing managed resources no-op
- create/update/delete/replace/drift/unknown/other 0
- `deletion_protection=true`
- `prevent_destroy=true`

を要求します。

BigQuery data mutation、schema DROP/backfill、production import、consumer変更はこのmoduleの責務外です。
