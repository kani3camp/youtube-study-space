# GCS の raw チャットを含むスナップショット削除手順

[廃止 Runbook](raw-live-chat-archive-retirement-runbook.md) における GCS 操作の追加条件をまとめた手順書です。

> [!IMPORTANT]
> 混合 Firestore エクスポートには `live-chat-history` だけでなく `users` / `user-activities` / `order-history` も含まれるため、raw kind だけの部分削除やバケット全体への短期ライフサイクルは使いません。削除対象は raw を含む **最上位スナップショットプレフィックス全体** であり、その中の古い業務データバックアップも同時に失われます（新しいクリーンなスナップショットと共有エクスポート基盤は維持します）。

---

## Apply 前の必須条件

1. 対象環境の照合: 環境・認証プロジェクト・バケット設定を毎回照合します（DEV の観測結果を PROD へ流用しません）。
2. 流入停止の確認: 配備済みの producer / export と定期実行コレクションを確認し、raw データの再流入や予期しない再配備がないことを確認します。
3. クリーン世代の確認: 事前の読み取り専用 `preview` で、Firestore raw が `0` 件、バケットの versioning が無効、最後の raw スナップショットより新しいクリーンなスナップショットが 7 世代以上、最新エクスポートが 72 時間以内であることを確認します。
4. オブジェクト属性の棚卸し: バケットの location / storage class / soft delete / retention / lock、および全候補オブジェクトの generation・retention expiration・temporary/event hold・object retention 設定を非公開棚卸しに固定し、プレフィックス内の **すべてのオブジェクト** が削除可能（eligible）であることを確認します。
5. 復元可能性と費用レビュー: クリーンなスナップショットから復元できること、失う古いバックアップの範囲、復旧経路、完全削除（hard-delete）時期、リクエスト費用・soft-delete 保管費・COLDLINE 早期削除費用をレビューし、別途削除承認を得ます（Production の COLDLINE / 93 日保持は維持し、個々の実メタデータから判定します。retention 経過だけで storage-class 料金がなくなると仮定しないでください）。

> [!CAUTION]
> 現在の `privacy-gcs-admin` は versioning / soft delete とオブジェクト名によるプレフィックスを確認しますが、retention / hold やスナップショットの完全性・復元可能性は検証しません。また全 raw プレフィックスを一括対象とし、一部のみ削除可能な状態（wave）を選べません。`ready_to_apply=true` だけで削除を実行せず、一部のみ eligible な状態では一括 apply を使わないでください（retention / hold を外したり、部分削除をリトライループで回したりせず、必要な対応・レビューを先に行います）。

---

## ガード付きコマンドの実行

`system/` ディレクトリから、[対象と実行境界](raw-live-chat-archive-retirement-runbook.md#対象と実行境界) の値を明示して実行します。

```sh
# 読み取り専用の事前確認（raw 件数・候補数・クリーン世代数・安全性チェック・確認トークンを出力）
go run ./cmd/privacy-gcs-admin preview <environment> <project-id> <bucket>
```

前節の追加条件と人間の承認がすべて揃った場合のみ、直前の `preview` 出力値を使って適用します（本番環境では `--allow-production` も必須）。

```sh
go run ./cmd/privacy-gcs-admin apply \
  <environment> <project-id> <bucket> \
  --expected-prefixes <preview-count> \
  --expected-objects <preview-count> \
  --confirm '<exact-preview-token>'
```

実行時は直前に再スキャンし、対象・raw 件数・クリーン世代・件数・トークンのいずれかが変化していたら停止します。各プレフィックス内では non-raw オブジェクトを先に削除し、最後まで raw マーカーを残すため部分失敗後も再発見できます（ただし retention を考慮した wave 分割の代替にはなりません）。

---

## 結果の確認と記録

- 削除状態の区別: 本番名前空間から raw プレフィックスが消えた状態（live 削除成功）と、バケットの実メタデータ上の soft-delete 期間経過による完全削除（hard deletion）を区別して記録します。`preview` で raw プレフィックスが 0 件なら削除を繰り返しません。
- 公開記録の範囲: 公開記録には集約件数・バイト数、eligibility 範囲、保持バックアップのカバー範囲、費用、非公開マニフェスト参照のみを残し、生オブジェクト名・設定全文・トークン・業務データは公開ログや成果物に出しません。
- 移行完了後は一時的な `DELETE` 機能を撤去します（一般のバックアップ保持ポリシーは raw アーカイブ廃止とは独立して管理します）。
