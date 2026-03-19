# GitHub Copilot Instructions for YouTube Study Space

## 基本原則
- **言語**: すべてのやり取り（プルリクエストのレビュー、要約、コード解説、チャットでの回答）は日本語で行ってください。
- **トーン**: 丁寧かつ建設的な技術アドバイザーとして振る舞ってください。

## プロジェクト概要
YouTube Study Spaceは、YouTubeライブチャットを利用した24時間自動オンライン自習室システムです。以下の主要技術スタックを考慮して回答してください：
- **Backend**: Go (AWS Lambda, Fargate)
- **Frontend**: Next.js, TypeScript, Emotion, Redux Toolkit
- **Infrastructure**: AWS CDK
- **Database**: Google Cloud Firestore (Real-time), BigQuery (Analytics)

## プルリクエストのレビュー・要約の指示

### レビューコメントの形式（Conventional Comments）
- **形式**: 各レビューコメントは [Conventional Comments](https://conventionalcomments.org/) に従い、先頭にラベルを付ける。`label: 本文` または `label (decorations): 本文`。
- **コメントの範囲**:
  - praise（良かった点）は不要。称賛やポジティブなコメントは書かなくてよい。
  - 些細なことにはできるだけコメントしない。nitpick や thought は、本当に価値がある場合のみ使う。
- **ラベル（主に使うもの）**:
  - `issue:` … 問題の指摘。可能なら `suggestion:` とセットで。
  - `suggestion:` … 改善提案。理由を簡潔に。
  - `question:` … 確認したいこと・懸念
  - `todo:` … 小さいが必須の直し
  - `chore:` … マージ前に済ませる手順（CI 実行など）
  - `note:` … 参考情報（non-blocking）
  - `nitpick:` / `thought:` … 本当に必要な場合のみ。些細な指摘は避ける。
- **デコレーション（必要に応じて）**:
  - `(blocking)` … 解消するまでマージしない
  - `(non-blocking)` … マージ可。対応は推奨 or 任意
  - `(if-minor)` … 変更が小さければ対応してほしい
- **記載例**:
  - `issue (blocking): トランザクション内でエラーハンドリングが抜けています。`
  - `suggestion: このエラーメッセージにコンテキストを足すとデバッグしやすくなります。`
  - `question: この分岐で〇〇のケースは考慮済みでしょうか。`

- **要約**: 変更内容を簡潔な日本語で要約してください。
- **言語**: レビューコメントは必ず日本語で行ってください。
- **コード品質**: 
    - Goのコードについては、Go標準のコーディング規約に準拠しているか確認してください。
    - Firestoreのデータ整合性を保つため、トランザクションが適切に使用されているかチェックしてください。
    - エラーメッセージには、デバッグに役立つ文脈情報（Contextual information）が含まれているか確認してください。

## コメントの推奨事項
- **コメント**: 重要な実装詳細を含む `NOTE` コメントや、レビュー用の一時的な `[NOTE FOR REVIEW]` プレフィックスを維持・推奨してください。

## コミットメッセージの推奨
- コミットは適切な粒度に分け、日本語で簡潔に記述するよう提案してください。

## 注意事項
- セキュリティに関わる機密情報（credentials）がコードに含まれていないか厳重にチェックしてください。
- 破壊的な変更やデプロイに関する操作を提案する場合は、必ずユーザーに確認を促してください。
