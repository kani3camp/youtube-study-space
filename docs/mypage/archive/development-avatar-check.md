# DEV channel avatar check

## Task contract

Goal: DEV MyPageでYouTubeチャンネル画像を読み込めるCSPを配信する。
Invariants: 許可元は確認したHTTPSの完全一致originのみ。script/connect/frame、認証・App Check・Origin、画像失敗時の代替アイコンとアカウント操作を維持する。個別の画像URL・アカウント情報は公開しない。
Non-goals: Production変更、画像proxy/保存、YouTube metadataや履歴の変更、dev/mainへのmerge。
Acceptance: image inventory不足をpreflightが拒否し、DEVの実HTTP CSPが確認済み2originをimg-srcに含む。他の配信設定と静的ファイルを維持する。実ログイン画像の表示は別途ブラウザで確認する。
Verification: Hosting/release-config回帰テスト、frontend lint/type/test/build、CI、公開HTTP headerと配信ファイルのreadback。実表示を確認できない場合は未確認と記録する。

## Findings (2026-10-21 UTC)

- DEV `/mypage` の公開HTTP responseは `img-src 'self' https://www.gstatic.com/recaptcha/`。外部チャンネル画像を許可していない。
- リポジトリREADMEに載る[公開チャンネル](https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg)を未認証HTTPで取得し、画像URLのoriginとして `https://yt3.ggpht.com` と `https://yt3.googleusercontent.com` を確認。既存monitor設定も前者を指定している。個別URL・response本文は保存しない。
- AccountAvatarはHTTPS画像の読み込み失敗で代替アイコンへ切り替わる。今回の欠落はCSP inventoryにあり、代替アイコンの条件を緩めない。
- Googleアカウントoriginのブラウザ観測は自動承認で拒否されたため、その画面へアクセスせず公開配信設定とsourceを調査した。実ユーザー画像のhost・読み込み結果は未確認。

## Deployment

Source修正・DEV配信・実HTTP readbackはpending。既存releaseのファイルとCSPの他directiveを維持し、レビューしたimage inventoryから候補を再生成して配信する。
