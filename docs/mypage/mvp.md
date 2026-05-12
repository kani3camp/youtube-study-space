# マイページMVP 仕様・API仕様

## 目的

YouTubeライブチャット上の `!info` 相当の情報を、Web上のマイページから確認できるようにする。

初期版では高機能なダッシュボードではなく、ログインしたユーザー本人が現在状態と作業時間を確認できる最小構成にする。

このドキュメントでは、マイページMVPの画面仕様、API仕様、フロントエンド構成、認証方式の前提、エラー時の表示方針を定義する。


## 基本方針

マイページMVPは、オンライン作業部屋の業務データに対して read-only とする。

マイページから入室、退室、休憩、作業内容変更、注文、ユーザー設定変更など、オンライン作業部屋の状態を変更する操作は行わない。

ただし、認証・連携に必要な `firebaseUid` と `youtubeChannelId` のサーバー側マッピングは、マイページ利用の前提情報として作成・更新する。

MVPでは、既存のオンライン作業部屋で記録済みのユーザー情報、作業時間、現在の席情報を参照して表示する。

YouTubeメンバーかどうかの判定はMVPには含めない。

ただし、現在座っている席が通常席かメンバー席かを表す `isMemberSeat` は、既存の席情報として返してよい。

## スコープ

### 画面表示項目

登録済みユーザーには、次の情報を表示する。

- YouTubeチャンネル情報
- 今日の作業時間
- 累計作業時間
- 現在の入室状態
- 現在の席番号
- 現在の作業内容

### ユーザー状態

マイページでは、最低限次の状態を扱う。

| 状態 | 説明 | UI方針 |
| --- | --- | --- |
| 未ログイン | Firebase Auth のログイン状態がない | ログイン導線を表示する |
| 認証処理中 | Googleログイン処理中 | ローディングを表示する |
| 登録済み・未入室 | ユーザー情報はあるが、現在席がない | 作業時間と未入室状態を表示する |
| 登録済み・作業中 | 現在席があり、state が `work` | 作業中として席情報と作業内容を表示する |
| 登録済み・休憩中 | 現在席があり、state が `break` | 休憩中として席情報と休憩中の作業内容を表示する |
| 未登録 | Google/YouTube認証は成功したが、オンライン作業部屋のユーザー情報がない | 利用案内を表示する |
| エラー | API、認証、通信、サーバー側処理で失敗 | 再試行導線つきのエラー表示にする |

## 非スコープ

MVPでは次を扱わない。

- YouTubeメンバー判定
- メンバー限定表示
- 作業履歴グラフ
- 月別・週別統計
- ランキング表示
- マイページ上からの入室・退室・休憩・作業内容変更
- 注文操作
- ユーザー設定変更
- 管理者画面
- 通知設定
- PWA対応
- ネイティブモバイル対応

## UI方針

### レイアウト

スマホ基準の1カラムUIを基本とする。

PCやタブレットでも、初期版では基本的に1カラムのまま表示する。

広い画面では、画面全体にコンテンツを広げず、読みやすい最大幅を設定して中央寄せする。

目安として、メインコンテンツ幅は `720px` から `960px` 程度に制限する。

複雑なサイドバーや多カラムダッシュボードは導入しない。

### 表示単位

情報はカード単位で縦に並べる。

推奨する初期構成は次の通り。

1. ページヘッダー
2. YouTubeチャンネルカード
3. 今日の作業時間カード
4. 累計作業時間カード
5. 現在の状態カード
6. 現在の作業内容カード
7. ログアウト導線

PC表示で横並びにするとしても、今日の作業時間と累計作業時間のサマリーカード程度に留める。

### 表示フォーマット

APIでは作業時間を秒数で返す。

フロントエンドで `h` / `m` 表記へ整形する。

例:

- `0m`
- `25m`
- `1h 05m`
- `123h 45m`

時刻はAPIでは RFC 3339 形式の文字列で返す。

フロントエンドでは必要に応じてJST表示へ整形する。

## フロントエンド構成

`mypage/` 配下に、TanStack Router を使った独立アプリを作る。

既存の `youtube-monitor/` や `docs-site/` には混ぜない。

### ルーティング

`mypage/` アプリでは、最低限次のルートを用意する。

| Route | 用途 |
| --- | --- |
| `/` | マイページ本体 |
| `/login` | ログイン開始、またはログイン説明画面 |
| `/auth/callback` | 必要な場合の認証後戻り先 |
| `/logout` | ログアウト |

Firebase Auth を使うため、OAuth callback 専用のサーバールートが必須とは限らない。

ただし、ログイン後の戻り先や将来の認証フロー拡張を考慮し、フロントエンド上の `/auth/callback` 相当のルートは用意してよい。

ルート名は実装時に調整してよいが、MVPでは上記の責務を満たすことを必須とする。

## 画面仕様

### 未ログイン状態

未ログインの場合は、マイページ本文の代わりにログイン導線を表示する。

表示内容:

- サービス説明
- Google / YouTube 連携でログインするボタン
- 取得する情報の簡単な説明

文言方針:

- チャンネル情報と作業時間を表示するためにYouTube連携が必要であることを説明する
- マイページMVPでは書き込み操作をしないことを明示する

### ログイン済み・登録済み状態

`status: "ok"` の場合は、APIレスポンスに基づいてユーザー情報と作業情報を表示する。

表示内容:

- チャンネル名
- チャンネルアイコン
- 今日の作業時間
- 累計作業時間
- 現在の入室状態
- 現在の席番号
- 現在の作業内容

### ログイン済み・未登録状態

`status: "not_registered"` の場合は、YouTube連携済みだがオンライン作業部屋の利用履歴が見つからない状態として扱う。

表示内容:

- チャンネル名
- チャンネルアイコン
- 未登録であることの説明
- YouTubeライブ配信でコマンドを使って入室するとマイページに情報が表示される旨の案内
- YouTubeライブ配信への導線

APIとしては単純な `404` ではなく、UIが扱いやすい `status: "not_registered"` を返す。

### 未入室状態

登録済みだが現在入室していない場合は、`currentSeat: null` を返す。

UIでは、今日の作業時間と累計作業時間は表示しつつ、現在状態は「未入室」として表示する。

作業内容カードは非表示にするか、「現在入室していません」と表示する。

### 作業中状態

`currentSeat.state` が `work` の場合は、現在状態を「作業中」として表示する。

作業内容には `currentSeat.workName` を表示する。

`workName` が空の場合は、「作業内容未設定」などの代替表示を行う。

### 休憩中状態

`currentSeat.state` が `break` の場合は、現在状態を「休憩中」として表示する。

作業内容には `currentSeat.breakWorkName` を優先して表示する。

`breakWorkName` が空の場合は、`workName` を表示するか、「休憩中」と表示する。

MVPでは、休憩終了予定時刻の表示は任意とする。

## 認証方式

### 認証フロー

MVPの認証フローは次の通りとする。

1. フロントエンドで Firebase Auth の Google provider を使ってログインする
2. Google provider に `https://www.googleapis.com/auth/youtube.readonly` scope を追加し、YouTube Data API 用のアクセストークンを取得する
3. フロントエンドは、Firebase ID token と YouTube access token をバックエンドの YouTube連携確定APIへ送る
4. バックエンドは Firebase ID token を検証し、Firebase UID を確定する
5. バックエンドは YouTube access token を使って YouTube Data API の `channels.list` を呼び出し、認証済みGoogleアカウント本人の YouTube channel ID を取得する
6. バックエンドは `firebaseUid` と `youtubeChannelId` の対応関係をサーバー側に保存する
7. 以降のマイページAPIでは、Firebase ID token から Firebase UID を検証し、サーバー側に保存済みの対応関係から YouTube channel ID を決定する
8. 決定した YouTube channel ID を既存システム上のユーザーIDとして `UserDoc` や現在席情報を参照する

### 方針

認証には Firebase Auth を使う。

フロントエンドでは Firebase Auth の Google provider を使ってログインし、YouTube Data API を読むために追加 scope として `https://www.googleapis.com/auth/youtube.readonly` を要求する。

バックエンドAPIは、フロントエンドから送られた Firebase ID token を検証し、認証済みユーザーとして扱う。

MVPでは、Firebase Auth の UID ではなく、YouTube Data API から取得した YouTube channel ID を既存システム上のユーザーIDとして扱う。

ただし、YouTube channel ID はフロントエンドから送られた値を信頼してはならない。

バックエンドが YouTube access token を使って YouTube Data API を呼び出し、サーバー側で YouTube channel ID を確定する。

### OAuth scope

MVPで明示的に追加する OAuth scope は次とする。

- `https://www.googleapis.com/auth/youtube.readonly`

Googleログインに必要な基本的な profile / email 取得は Firebase Auth の Google provider に委ねる。

YouTube channel ID は Firebase Auth の ID token だけでは取得できない前提とする。

### YouTube channel ID の確定

Firebase ID token だけでは、既存システム上のユーザーIDとして使う YouTube channel ID は決定できない。

そのため、MVPでは次の方式を採用する。

- フロントエンドは Firebase Auth の Google provider から YouTube access token を取得する
- フロントエンドは Firebase ID token と YouTube access token をバックエンドへ送る
- バックエンドは Firebase ID token を検証して Firebase UID を確定する
- バックエンドは YouTube access token を使って YouTube Data API を呼び出す
- バックエンドは API レスポンスから YouTube channel ID、チャンネル名、チャンネルアイコンを取得する
- バックエンドは Firebase UID と YouTube channel ID の対応関係をサーバー側に保存する

YouTube channel ID の確定時に呼び出す YouTube Data API は次の想定とする。

```http
GET https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true
Authorization: Bearer <youtube_access_token>
```

取得したチャンネル情報から、次を viewer 情報として扱う。

- `youtubeChannelId`: channel resource の `id`
- `displayName`: `snippet.title`
- `profileImageUrl`: `snippet.thumbnails` の利用可能なURL

### サーバー側マッピング

バックエンドは、Firebase UID と YouTube channel ID の対応関係をサーバー側に永続化する。

概念上は次のような情報を保持する。

```ts
interface MyPageAuthLink {
  firebaseUid: string
  youtubeChannelId: string
  displayName: string
  profileImageUrl: string
  linkedAt: string
  updatedAt: string
}
```

`youtubeChannelId` は、このサーバー側マッピングまたはバックエンド自身が YouTube Data API で取得した値のみを信頼する。

フロントエンドから送られた `youtubeChannelId` は、表示補助やログ用途であっても、認可判断やユーザーID決定には使わない。

MVPでは Custom Claims ではなく、サーバー側の永続マッピングを優先する。

理由は、YouTube channel ID やチャンネル表示情報の更新、再連携、連携解除を扱いやすくするためである。

### API認証

フロントエンドは Firebase Auth のログイン状態を保持する。

バックエンドAPI呼び出し時は、Firebase ID token を `Authorization: Bearer <id_token>` で送る。

バックエンドは Firebase Admin SDK などで ID token を検証し、Firebase UID を確定する。

`GET /mypage/me` は、Firebase UID からサーバー側マッピングを参照し、対応する YouTube channel ID を決定する。

マッピングが存在しない場合は、YouTube連携が未完了として扱う。

### 信頼境界

| 値 | 送信元 | 信頼可否 | 用途 |
| --- | --- | --- | --- |
| Firebase ID token | フロントエンド | バックエンドで検証後のみ信頼する | Firebase UID の確定 |
| Firebase UID | Firebase ID token 検証結果 | 信頼する | サーバー側マッピングのキー |
| YouTube access token | フロントエンド | そのまま保存せず、YouTube API 呼び出しにのみ使う | YouTube channel ID のサーバー側確定 |
| YouTube channel ID | バックエンドが YouTube API から取得 | 信頼する | 既存 `UserDoc` / 席情報の参照キー |
| YouTube channel ID | フロントエンド申告値 | 信頼しない | 認可判断・ユーザーID決定には使わない |
| `X-Client-*` ヘッダー | フロントエンド | 信頼しない | ログ分析・デバッグ補助のみ |

### ログアウト

ログアウトでは Firebase Auth のサインアウトを行う。

Googleアカウント側の連携解除まではMVPでは扱わない。

### YouTube連携が必要な場合の再認可

`GET /mypage/me` が `409 link_required` を返した場合、フロントエンドは YouTube連携が未完了、またはサーバー側マッピングが存在しない状態として扱う。

この場合、フロントエンドはユーザーに再ログインまたは再認可を促し、Firebase Auth の Google provider で `https://www.googleapis.com/auth/youtube.readonly` scope を要求し直す。

ユーザーが Google の同意画面で YouTube 情報の読み取り許可を付与しなかった場合、YouTube channel ID をサーバー側で確定できないため、マイページ表示は完了できない。

その場合は、画面上で「YouTubeチャンネル情報を確認するため、Googleの確認画面でYouTube情報の読み取りを許可してください」のように案内し、再度ログイン/再認可できる導線を表示する。

フロントエンドは YouTube access token を永続保存しない。`POST /mypage/auth/youtube-link` に必要な YouTube access token は、ログイン直後または再認可直後に取得したものを使用する。

## API仕様

### API共通リクエストメタ情報

マイページAPIでは、認証情報とは別に、観測・デバッグ・問い合わせ対応のためのフロントエンド由来メタ情報をできるだけ付与する。

ただし、これらの値は認可判断には使わない。バックエンド側では、ログ分析・障害調査・互換性確認のための補助情報として扱う。

#### 必須ヘッダー

APIの認証に必須のヘッダーは `Authorization` のみとする。

| Header | 例 | 用途 |
| --- | --- | --- |
| `Authorization` | `Bearer <firebase_id_token>` | Firebase ID token によるAPI認証 |

`Authorization` がない、または Firebase ID token が無効な場合、APIは `401 unauthorized` を返す。

#### 推奨ヘッダー

`X-Client-*` を含むフロントエンド由来メタ情報は、送信を推奨するが必須ではない。

これらが未送信でもAPIは正常動作する。バックエンドのログでは、未送信の値を `unknown` または空値として扱う。

| Header | 例 | 用途 |
| --- | --- | --- |
| `X-Client-App` | `mypage` | 呼び出し元フロントアプリの識別 |
| `X-Client-Version` | `0.1.0` / `2026.05.12.1` / short SHA | フロントアプリのバージョン識別 |
| `X-Client-Request-Id` | `01J...` | フロントからAPI呼び出し単位で発行するリクエストID |
| `X-Client-Build-Time` | `2026-05-12T10:30:00Z` | フロントアプリのビルド日時。RFC 3339 |
| `User-Agent` | ブラウザ既定値 | ブラウザ・OS・端末種別の概略把握 |
| `Sec-CH-UA` | ブラウザが送信する値 | User-Agent Client Hints。ブラウザ識別の補助 |
| `Sec-CH-UA-Mobile` | `?0` / `?1` | モバイル相当の表示環境かどうかの補助 |
| `Sec-CH-UA-Platform` | `"macOS"` / `"Android"` | OS / platform の補助 |
| `Accept-Language` | `ja,en-US;q=0.9` | 将来の表示言語・問い合わせ調査の補助 |
| `X-Client-Timezone` | `Asia/Tokyo` | フロント側表示タイムゾーンの確認 |

`X-Client-Version` は、package version、Git short SHA、またはデプロイ番号のいずれかを使う。

MVPでは、フロント側の実装負荷を抑えるため、`X-Client-Version` に Git short SHA またはビルド時に埋め込んだ任意のビルドIDを入れればよい。

`X-Client-Request-Id` は、ブラウザ上でリクエストごとに生成する。バックエンドのログにも同じ値を出し、フロントのエラー表示や問い合わせログと突合できるようにする。

`X-Client-Build-Time` は、ビルド時刻をUTCのRFC 3339文字列で埋め込める場合のみ送る。

端末判定は、基本的には `User-Agent` と低エントロピーの `Sec-CH-UA-*` をログに残せば十分とする。

`Sec-CH-UA-*` はブラウザや環境によって送られないことがあるため、必須にはしない。

`X-Client-Timezone` は、`Intl.DateTimeFormat().resolvedOptions().timeZone` で取得したIANA timezoneを送る想定とする。ただし、作業時間の集計仕様は引き続きJST基準とし、この値で業務ロジックを変えない。

#### 原則メタ情報として送らない・ログに残さない情報

通常のAPIリクエストでは、次の情報をフロントエンド由来メタ情報として送らない。また、認証や連携処理に必要な場合でもログには残さない。

| 情報 | 理由 |
| --- | --- |
| 画面サイズ / viewport | UI不具合調査では有用だが、通常ログには過剰 |
| 端末の詳細機種名 | 指紋化リスクが高く、MVPでは不要 |
| CPU / メモリ / ネットワーク情報 | 調査価値に対してプライバシー上の情報量が大きい |
| 緯度経度などの位置情報 | マイページMVPの仕様上不要 |
| Firebase ID token / Firebase refresh token / YouTube access token | 認証・連携に必要な場所以外に含めず、ログに混入させないため |

画面サイズ、詳細な端末情報、ネットワーク情報などは、通常APIではなく、将来問い合わせ送信機能や明示的なデバッグレポート機能を作る場合にのみ検討する。

#### リクエスト例

```http
GET /mypage/me HTTP/1.1
Authorization: Bearer <firebase_id_token>
X-Client-App: mypage
X-Client-Version: a1b2c3d
X-Client-Build-Time: 2026-05-12T10:30:00Z
X-Client-Request-Id: 01HX0000000000000000000000
X-Client-Timezone: Asia/Tokyo
Accept-Language: ja
```

### エンドポイント

#### `POST /mypage/auth/youtube-link`

Firebase UID と YouTube channel ID の対応関係をサーバー側で確定する。

このAPIは、Firebase Auth のログイン直後、またはサーバー側マッピングが存在しない場合に呼び出す。

リクエスト:

```ts
interface LinkYouTubeRequest {
  youtubeAccessToken: string
}
```

認証:

- `Authorization: Bearer <firebase_id_token>` 必須
- body の `youtubeAccessToken` 必須

処理:

1. Firebase ID token を検証し、Firebase UID を確定する
2. `youtubeAccessToken` を使って YouTube Data API の `channels.list?part=snippet&mine=true` を呼び出す
3. 取得した YouTube channel ID と表示情報を Firebase UID に紐づけて保存する
4. YouTube access token は永続保存しない

レスポンス:

```ts
interface LinkYouTubeResponse {
  status: "ok"
  viewer: MyPageViewer
}
```

#### `GET /mypage/me`

認証済みユーザー本人のマイページ表示情報を返す。

`GET /mypage/me` は、Firebase ID token から Firebase UID を確定し、サーバー側に保存済みの `firebaseUid` と `youtubeChannelId` の対応関係を使ってユーザー情報を取得する。

フロントエンドから `youtubeChannelId` を渡してユーザーを指定することはできない。

### 認証

Firebase ID token 必須。

未ログイン、または ID token がない/無効な場合は `401 Unauthorized` を返す。

`GET /mypage/me` で Firebase UID と YouTube channel ID の対応関係が存在しない場合は、`401` ではなく `409 link_required` を返す。

フロントエンドは `409 link_required` を受け取った場合、保持済みの YouTube access token に依存せず、Googleログイン/再認可フローを実行して `https://www.googleapis.com/auth/youtube.readonly` scope 付きの YouTube access token を取り直す。

その後、取得した YouTube access token を使って `POST /mypage/auth/youtube-link` を呼び出し、サーバー側マッピングを作成・更新する。

### `GET /mypage/me` レスポンス型

`GET /mypage/me` の `200 OK` では、成功系レスポンスとして `MyPageMeSuccessResponse` を返す。

エラー時は `MyPageErrorResponse` を返す。

```ts
type MyPageMeResponseBody =
  | MyPageMeSuccessResponse
  | MyPageErrorResponse

type MyPageMeSuccessResponse =
  | MyPageOkResponse
  | MyPageNotRegisteredResponse

interface MyPageViewer {
  youtubeChannelId: string
  displayName: string
  profileImageUrl: string
}

interface MyPageOkResponse {
  status: "ok"
  viewer: MyPageViewer
  stats: {
    dailyWorkSec: number
    cumulativeWorkSec: number
  }
  currentSeat: MyPageCurrentSeat | null
}

interface MyPageCurrentSeat {
  seatId: number
  isMemberSeat: boolean
  state: "work" | "break"
  workName: string
  breakWorkName: string
  startedAt: string
  until: string
}

interface MyPageNotRegisteredResponse {
  status: "not_registered"
  viewer: MyPageViewer
}
```

### HTTPステータスとレスポンスボディ

| Endpoint | HTTP status | Body type | 説明 |
| --- | --- | --- | --- |
| `GET /mypage/me` | `200` | `MyPageMeSuccessResponse` | 登録済み、または未登録状態を正常系として返す |
| `GET /mypage/me` | `401` | `MyPageErrorResponse` | 未ログイン、ID token なし、ID token 無効 |
| `GET /mypage/me` | `409` | `MyPageErrorResponse` | YouTube連携確定が必要 |
| `GET /mypage/me` | `403` / `429` / `500` / `502` | `MyPageErrorResponse` | 認可失敗、レート制限、内部エラー、外部APIエラー |
| `POST /mypage/auth/youtube-link` | `200` | `LinkYouTubeResponse` | YouTube連携確定成功 |
| `POST /mypage/auth/youtube-link` | `401` / `403` / `429` / `500` / `502` | `MyPageErrorResponse` | 認証失敗、認可失敗、レート制限、内部エラー、外部APIエラー |

### フィールド定義

| Field | Type | 説明 |
| --- | --- | --- |
| `status` | `"ok" \| "not_registered"` | マイページ表示状態 |
| `viewer.youtubeChannelId` | `string` | 認証済みユーザー本人のYouTube channel ID |
| `viewer.displayName` | `string` | YouTubeチャンネル名 |
| `viewer.profileImageUrl` | `string` | YouTubeチャンネルアイコンURL |
| `stats.dailyWorkSec` | `number` | 今日の作業時間。秒単位 |
| `stats.cumulativeWorkSec` | `number` | 累計作業時間。秒単位 |
| `currentSeat` | `MyPageCurrentSeat \| null` | 現在入室していない場合は `null` |
| `currentSeat.seatId` | `number` | 席番号 |
| `currentSeat.isMemberSeat` | `boolean` | 現在座っている席がメンバー席かどうか |
| `currentSeat.state` | `"work" \| "break"` | 現在の席状態 |
| `currentSeat.workName` | `string` | 作業中の作業名 |
| `currentSeat.breakWorkName` | `string` | 休憩中の作業名 |
| `currentSeat.startedAt` | `string` | 現在の状態が開始した時刻。RFC 3339 |
| `currentSeat.until` | `string` | 自動退室の期限。RFC 3339 |


### 未登録レスポンス

```json
{
  "status": "not_registered",
  "viewer": {
    "youtubeChannelId": "UCxxxxxxxxxxxxxxxxxxxxxx",
    "displayName": "example channel",
    "profileImageUrl": "https://example.com/avatar.jpg"
  }
}
```

未登録とは、Google / YouTube 認証には成功しているが、既存のユーザー情報が見つからない状態を指す。

この状態はアプリケーション上の正常系として扱い、HTTPステータスは `200 OK` とする。

## エラー仕様

### エラーレスポンス型

```ts
interface MyPageErrorResponse {
  status: "error"
  error: {
    code: MyPageErrorCode
    message: string
  }
}

type MyPageErrorCode =
  | "unauthorized"
  | "link_required"
  | "upstream_auth_error"
  | "youtube_channel_not_found"
  | "forbidden"
  | "rate_limited"
  | "internal_error"
```

### HTTPステータス

| HTTP status | code | 用途 | UI方針 |
| --- | --- | --- | --- |
| `401` | `unauthorized` | 未ログイン、ID token なし、ID token 無効 | ログイン導線を表示 |
| `409` | `link_required` | Firebase UID と YouTube channel ID のサーバー側マッピングがない | Google再ログイン/再認可で `youtube.readonly` scope 付き access token を取り直し、YouTube連携確定APIを呼び出す |
| `403` | `forbidden` | 認可失敗、想定外の権限不足 | 再ログイン導線を表示 |
| `429` | `rate_limited` | レート制限 | 時間を置いて再試行する案内 |
| `500` | `internal_error` | サーバー内部エラー | 再試行導線つきの汎用エラー |
| `502` | `upstream_auth_error` | Firebase Auth / Google API など上流サービス側の認証・連携処理失敗。呼び出し側の認証失敗ではない | 再試行または再ログイン導線を表示 |
| `502` | `youtube_channel_not_found` | 認証済みアカウントにYouTubeチャンネルが見つからない | チャンネル作成または別アカウント案内 |

`not_registered` はエラーではないため、このエラー形式には含めない。

## データ取得方針

### ユーザー情報

バックエンドがサーバー側マッピングから決定した `youtubeChannelId` を既存システムのユーザーIDとして扱う。

既存ユーザー情報から次を取得する。

- 今日の作業時間
- 累計作業時間

既存モデル上は、今日の作業時間は `DailyTotalStudySec`、累計作業時間は `TotalStudySec` を参照する。

### 現在席情報

通常席とメンバー席の両方を検索し、認証済みユーザー本人の現在席を取得する。

どちらにも席がない場合は `currentSeat: null` とする。

通常席とメンバー席の両方に同一ユーザーがいる状態は不整合として扱う。

MVPでは、サーバー側でエラーログを出し、APIは `500 internal_error` を返す。

### リアルタイム作業時間

MVPの作業時間は `!info` 相当を目指す。

現在入室中の場合は、既存のリアルタイム累計計算ロジックを利用して、現在進行中の作業時間を反映した値を返す。

未入室の場合は、保存済みのユーザー累計値を返す。

## セキュリティ・プライバシー方針

- APIは認証済みユーザー本人の情報のみ返す
- 任意の `userId` や `youtubeChannelId` をクエリで指定して他人の情報を取得するAPIにはしない
- フロントエンドから送られた `youtubeChannelId` をユーザーIDとして信頼しない
- Firebase ID token と YouTube access token を混同しない
- YouTube access token は YouTube channel ID のサーバー側確定にのみ使い、永続保存しない
- YouTube access token をブラウザの永続ストレージに保存しない
- バックエンドは Firebase ID token を検証してから処理する
- Firebase UID と YouTube channel ID の対応関係はサーバー側で永続化する
- MVPでは、オンライン作業部屋の状態を変更する書き込み操作は提供しない

## 実装メモ

### フロントエンド

- `mypage/` 配下に独立した TanStack Router アプリを作る
- `youtube-monitor/` や `docs-site/` には混ぜない
- Firebase Auth の Google provider を使う
- Google provider に `https://www.googleapis.com/auth/youtube.readonly` scope を追加する
- ログイン後に YouTube channel ID を取得する
- APIレスポンス型はフロントエンドにも共有しやすい形にする
- ローディング、未ログイン、未登録、未入室、エラーの状態表示を明示的に分ける

### バックエンド

- `GET /mypage/me` は read-only にする
- Firebase ID token を検証して認証済みユーザーを特定する
- YouTube channel ID を取得し、既存ユーザーIDとして扱う
- YouTube channel ID から既存 `UserDoc` を取得する
- 現在席は通常席・メンバー席を確認して取得する
- 入室中の場合はリアルタイム作業時間を反映する
- 未登録は `status: "not_registered"` で返す

## MVP完了条件

- Firebase Auth の Googleログインができる
- YouTube readonly scope を要求できる
- 認証済みユーザーの YouTube channel ID を取得できる
- 登録済みユーザーはマイページで `!info` 相当の情報を確認できる
- 未登録ユーザーには案内状態が表示される
- 未入室、作業中、休憩中が正しく表示される
- 初期版ではYouTubeメンバー判定を行わない
- 初期版ではマイページから書き込み操作を行わない
- APIレスポンス仕様がドキュメント化されている
- エラー時の表示方針がドキュメント化されている
