# マイページ Phase 1 API設計

この文書は、Phase 1 マイページAPIの**設計意図と意味論**を定義する。
wire contract（endpoint、request / response、HTTP status、error code、schema）は
[`openapi.yaml`](./openapi.yaml) を正とする。

## 正本の境界

- **Notion Current Canon**
  - 何を提供するか
  - 画面・状態別UX
  - 認証・アカウント方針
  - 作業時間・統計の意味
  - Privacy / Security
  - 技術アーキテクチャ原則
- **GitHub**
  - API wire contract
  - 実装設計
  - Approved Visual Reference
  - コード、テスト、CI、デプロイ状態

NotionとGitHubに同じrequest / response定義を二重管理しない。

## Phase 1 API surface

| Method | Path | 役割 |
| --- | --- | --- |
| `GET` | `/api/mypage/me` | 認証済み本人のマイページ表示スナップショット |
| `POST` | `/api/mypage/auth/youtube/preview` | YouTube候補チャンネルを取得。mappingは保存しない |
| `POST` | `/api/mypage/auth/youtube/confirm` | ユーザーが選択したチャンネルを再検証し、mappingを保存 |

Phase 1では、入室・退室・休憩・作業内容変更などのStudy Space状態変更APIは提供しない。

## 認証境界

- API認証主体はFirebase ID tokenから検証したFirebase UID
- `Authorization: Bearer <firebase_id_token>` を必須とする
- フロントエンドが申告したFirebase UIDは信頼しない
- YouTube channel IDを認可判断に直接使わない
- Firebase UID ↔ YouTube channel ID mappingはサーバー側で管理する
- YouTube access tokenはYouTube API呼び出しにだけ利用し、永続保存しない

## YouTube連携

OAuth成功だけでYouTubeチャンネルを確定しない。

```text
Google / YouTube認可
        |
        v
POST /youtube/preview
        |
        | mapping保存なし
        v
チャンネル確認画面
        |
        | ユーザーが明示選択
        v
POST /youtube/confirm
        |
        | Backendで候補を再取得・所有関係を再検証
        v
Firebase UID <-> YouTube channel mapping保存
```

### preview

`preview` は候補表示のためのread-only処理であり、mappingを書き換えない。

候補が1件でも確認画面は省略しない。

### confirm

`confirm` はフロントエンドから送られた `youtubeChannelId` をそのまま信頼しない。
同じYouTube access tokenで候補を再取得し、選択IDが候補に含まれることを確認してから保存する。

同一YouTube channelを別Firebase UIDが所有済みの場合は自動takeoverせず、
`409 channel_already_linked` とする。

## `GET /mypage/me`

1 requestで画面に必要なPhase 1データを1つのスナップショットとして返す。

### 登録状態

- `status: "ok"`
  - Study Spaceユーザーが存在する
  - `stats` を返す
  - 未入室なら `current: null`
- `status: "not_registered"`
  - YouTube連携は完了している
  - Study Spaceユーザーが存在しない
  - アプリケーション上の正常状態なので `200 OK`

mapping自体が存在しない場合は `409 youtube_link_required` とする。

### viewer

メイン画面に必要な以下だけを返す。

- `displayName`
- `profileImageUrl`

通常のマイページレスポンスではYouTube channel IDをUIへ返す必要はない。
チャンネル確認フローだけは選択対象を識別するためcandidateにchannel IDを含める。

### current

未入室は `null` で表す。`state: "out"` は作らない。

入室中は以下を返す。

- `roomType: "general" \| "member"`
- `seatNumber`
- `state: "work" | "break"`
- `workName`
- `stateStartedAt`
- `stateEndsAt`

`workName` は休憩中も保持する。休憩専用作業名はAPIに持たない。

#### 時刻の意味

`stateStartedAt`:
- work: 現在の作業状態を開始した時刻
- break: 休憩開始時刻

`stateEndsAt`:
- work: 現在の作業状態の終了予定 = 自動退室予定
- break: 休憩終了予定

現行ドメインモデルでは `SeatDoc.CurrentStateStartedAt` /
`SeatDoc.CurrentStateUntil` に対応する。

background処理の遅延等により `stateEndsAt < stats.calculatedAt` となることは許容する。
API側で勝手に時刻を丸めたり「未入室」へ変換せず、UIが終了予定超過状態を表現できるようにする。

## 作業統計

統計はJST固定。

| Field | 定義 |
| --- | --- |
| `todayWorkSec` | 当日00:00 JST〜`calculatedAt` のwork時間 |
| `weekWorkSec` | 月曜00:00 JST〜`calculatedAt` のwork時間 |
| `totalWorkSec` | 生涯work時間。現在work中なら進行中分を含む |
| `recentDays` | 今日 + 過去6暦日の7件。古い日→今日の順 |
| `calculatedAt` | realtime分を計算したサーバー側スナップショット時刻 |

break時間はすべてのwork統計から除外する。

現在work中の場合は進行中区間を加算する。
現在break中の場合、break開始後の時間は加算しない。

### 0と取得不能

正常な0秒と取得不能を混同しない。

`recentDays[].workSec = 0` は「正しく計算した結果0秒」の場合だけ返す。
必要な履歴を信頼して算出できない場合に、未知の値を0埋めしてはならない。

Phase 1で必要な統計スナップショットを信頼して生成できない場合は、
`503 stats_unavailable` とし、フロントエンドは以下の既存UXへ流す。

- 初回取得時: 初回取得失敗
- 過去に成功済み: 最後の成功データを保持した更新失敗

## エラーとUI

| API condition | HTTP / code | UIの主な扱い |
| --- | --- | --- |
| Firebase認証なし/無効 | `401 unauthorized` | ログイン |
| channel mappingなし | `409 youtube_link_required` | 再連携 |
| access token不正/scope不足 | `400 invalid_youtube_access_token` | 再認可 |
| YouTube channelなし | `422 youtube_channel_not_found` | 別アカウント/チャンネル案内 |
| confirm時の候補不一致 | `409 youtube_channel_mismatch` | 確認フローをやり直す |
| 別Firebase UIDが所有済み | `409 channel_already_linked` | 紐付け競合 |
| 統計を正しく生成不能 | `503 stats_unavailable` | 初回/更新エラー |
| Google / YouTube障害 | `502 upstream_error` | 再試行 |
| rate limit | `429 rate_limited` | 時間を置いて再試行 |

エラーメッセージ本文を認可判断やUI状態判定に使わず、`error.code` を使う。

## 現行 `feature/mypage` からの主な移行差分

現在の実装はPhase 1契約へ同期が必要。

| 現行 | Phase 1契約 |
| --- | --- |
| `POST /mypage/auth/youtube-link` で即保存 | `preview` と `confirm` に分割 |
| `breakWorkName` | 廃止。`workName` を休憩中も保持 |
| `dailyWorkSec` / `cumulativeWorkSec` | 今日 / 今週 / 累計 / 直近7日 |
| `isMemberSeat: boolean` | `roomType: "general" \| "member"` |
| `startedAt` / `until` | `stateStartedAt` / `stateEndsAt` |
| AWS Lambda / API Gateway前提の実装 | Current CanonのCloud Run + Firebase Hosting `/api/**` 方針へ同期 |

実装時は、`feature/mypage` を現行 `dev` のドメインモデルへ追従させてからAPI/backendを更新する。

## Phase 1で契約に含めないもの

- 入室、退室、休憩、作業内容変更
- Premium entitlement API
- 保存データ削除API
- Google OAuth revokeの即時検知API
- user-specific timezone
- WebSocket / realtime push

OAuth revokeを**いつ検知するか**はCurrent Canon上でも未確定のため、
`youtube_link_required` は「revokeをリアルタイム検知できる」という保証を含まない。

## 実装時の最低テスト

- tokenなし / 不正token
- Firebase UID AでBのデータを取得できない
- previewがmappingを変更しない
- confirm時にcandidateを再検証する
- channel ownership競合で自動takeoverしない
- work / break / 未入室
- break中も`workName`が保持される
- general / member room
- 今日 / 今週 / 累計 / 直近7日のJST境界
- 日跨ぎwork segment
- breakを統計に加算しない
- 正常0と統計取得失敗を区別する
- `stateEndsAt`超過スナップショットを正常に返せる
