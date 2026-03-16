# FOSSA廃止対応計画

## 背景

このリポジトリでは、現状 FOSSA に関する痕跡が主に以下に残っている。

- `README.md` の FOSSA バッジ
- `ATTRIBUTIONS.md` の FOSSA 生成物
- `AGENTS.md` / `CLAUDE.md` の「FOSSA を利用している」という説明

一方で、リポジトリ内の GitHub Actions workflow には FOSSA 実行ステップは見当たらない。そのため、実際のスキャンやステータス連携が継続している場合は、GitHub App や FOSSA 側の外部設定で動いている可能性が高い。

## 目的

- FOSSA への依存をリポジトリ運用から全面的に取り除く
- README / 開発文書 / CI 表示 / 外部連携の整合を取る
- 必要であれば、FOSSA の代替なしでも成立するライセンス管理運用へ移行する

## 対応方針

### 1. リポジトリ内の FOSSA 参照を撤去する

対象:

- `README.md`
  - FOSSA バッジを削除する
  - `ATTRIBUTIONS.md` への案内文を残すか削除するかを判断する
- `AGENTS.md`
  - Third-Party 記述から FOSSA を削除する
- `CLAUDE.md`
  - Third-Party 記述から FOSSA を削除する

完了条件:

- リポジトリ内に FOSSA への直接参照が残らない
- README の説明と実際の運用が一致している

### 2. `ATTRIBUTIONS.md` の扱いを決定する

判断が必要な点:

- FOSSA 生成物である `ATTRIBUTIONS.md` を今後も残すか
- 残す場合、更新停止した静的ドキュメントとして扱うか
- 削除する場合、OSS attribution をどこで管理するか

推奨:

- 「FOSSA をやめる」だけが目的で、代替ツールをまだ決めない場合は、まず `ATTRIBUTIONS.md` を削除して README の案内も外す
- もし attribution 一覧を保持したい要件があるなら、別手段で再生成できる運用を決めてから差し替える

完了条件:

- attribution の保管場所が明確になる
- README 上の案内が実体と一致する

### 3. リポジトリ外の FOSSA 連携を停止する

確認対象:

- GitHub App として FOSSA がインストールされていないか
- PR ステータスチェックに FOSSA が required check として残っていないか
- FOSSA の Webhook / repository integration が有効のままになっていないか
- FOSSA 用トークンやシークレットが GitHub / CI 環境に残っていないか
- FOSSA プロジェクトを archive または delete すべきか

完了条件:

- PR 上で FOSSA ステータスが出なくなる
- branch protection に FOSSA 依存が残らない
- 運用上不要なシークレットや外部連携が整理される

### 4. ライセンス管理運用の着地点を決める

選択肢:

- 当面は `LICENSE` と依存関係の通常管理のみで運用する
- 別のライセンス確認手段へ移行する
- attribution ファイルのみ別手段で維持する

判断観点:

- 配布物に attribution 同梱が必要か
- CI で自動検査が必要か
- 運用コストをどこまで許容するか

完了条件:

- FOSSA 廃止後の最低限の運用ルールが決まる

## 実施ステップ案

### Phase 1: 方針確定

- `ATTRIBUTIONS.md` を残すか削除するか決める
- 代替のライセンス確認方法を入れるか決める
- GitHub / FOSSA の管理権限者を確認する

### Phase 2: リポジトリ内修正

- `README.md` から FOSSA バッジと関連文言を削除または調整
- `AGENTS.md` と `CLAUDE.md` から FOSSA 記述を削除
- 方針に応じて `ATTRIBUTIONS.md` を削除または説明付きで維持

### Phase 3: 外部設定の停止

- GitHub App / required status check / secrets を整理
- FOSSA 側の repository integration を停止

### Phase 4: 検証

- `rg -n "FOSSA|fossa" .` で痕跡が残っていないことを確認する
- GitHub 上で PR の required checks を確認する
- README の見え方とリンク切れを確認する

## この計画に基づく実装PRの推奨構成

1. **PR 1: リポジトリ内の参照撤去**
   - README / AGENTS / CLAUDE / `ATTRIBUTIONS.md` の整理
2. **PR 2: 外部設定の停止**
   - GitHub App / branch protection / secret / FOSSA 側設定の整理

外部設定の変更履歴をコードレビューに残しづらいため、PR 1 の説明欄に PR 2 相当の運用作業チェックリストを含めるのがよい。

## 今回の調査で確認できたこと

- リポジトリ内 workflow には FOSSA 実行ステップがない
- README には FOSSA バッジと `ATTRIBUTIONS.md` への導線がある
- `ATTRIBUTIONS.md` は FOSSA 生成物である可能性が高い
- 開発者向け文書に FOSSA 利用前提の説明が残っている
