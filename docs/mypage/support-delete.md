# Support deletion B: workflow と運用 gate

この変更は、確認済みの削除受付を1つの永続的な実行に結び付けるワークフローを提供する。Firestoreのチェックポイント、ソースの許可リストに従うアダプター、非公開の運用CLI、障害後の再開試験を含む。CLIの既定はオフラインの計画確認である。実接続の初期化処理はなく、`--execute`は`LIVE_ADAPTERS_UNAVAILABLE`で拒否する。モックの完了は実データの削除完了を意味しない。実ランタイム・Auth・BQ・バックアップ・ベンダー・ログのアダプターが提供する保証は未確認であり、実削除の完了条件を満たしたとは扱わない。

[元チャット保存の廃止](../privacy/raw-live-chat-archive-retirement-runbook.md)と[混合スナップショットの処置](../privacy/gcs-raw-chat-cleanup.md)の対象・整合性条件は変更しない。

## 実行仕様

入力は削除の操作意図であり、本人確認の証拠、停止、実行中処理の完了、ストアの不存在を証明しない。初回は同じトランザクションで、`support-requests`の本人確認済み削除受付、消費済みサポートOAuthの目的・環境・申請との対応、申請インデックス、対象・チャンネル・証拠を照合する。元のacceptedAtと、acceptedAtから7暦日後のdeleteByを保持する。再発行や再開でSLAをリセットしない。7日超過後の修復は期限内成功とせず、完了時刻から超過を監査する。

新たな本人確認は、既存のチャレンジ24時間・OAuth 10分の仕様に従う、申請に結び付いた対話認証である。実行開始に任意の最大経過時間を追加しない。初回にOAuth追跡記録がTTLで失われていれば、不明として拒否する。検証済み証拠の確保、privacyの有効化、実行の作成はアトミックに行う。同じ実行は結び付け済みの証拠で再開でき、途中でOAuth追跡記録が消えても再認証を強制しない。別の実行・対象・証拠・マニフェストへの流用は認めない。

`support-delete-executions/{executionRef}`と`support-delete-proof-claims/{proofRef}`はサーバー専用とする。チェックポイントは対象指定・所有者・リビジョン・制御世代・guardSince/cutoff・SLA・固定対象一覧のダイジェスト・カーソル・証跡列のダイジェストを持つ。TTL・リース期限・自動引き継ぎは設けない。worker停止後の所有者変更には、独立したランタイムアダプターが、同じ実行・リビジョン・世代の旧worker停止を確認した新しい推測不能な追跡情報が必要である。待ち時間やハンドラー件数0だけでは、停止・SDKコミット・トークン発行や配送の完了を証明できない。

全29段階の操作順序は固定し、入力から対象範囲を省略できない。

1. 対応する全Auth本人情報のセッションを失効する。
2. MyPageと旧システムの全実行系を停止し、両方の実行中処理の完了を待つ。
3. 専用の座席除去、主データ、Web、OAuth、サポート関連、旧マッピング、BQ、バックアップ・エクスポート、派生データ・ベンダー、基盤ログを消去する。Canon 05に従い、Authユーザーはこれらの消去後に削除する。
4. 停止中に、全11対象範囲の不存在と旧データが再生成されないことを再確認する。
5. マニフェストに記録した開始前の有効状態へ、両方の実行系を戻す。古いキュー・コールバック・発行結果・スナップショット・import・restoreの再実行は認めない。
6. 現在のrequestRefと最新の制御世代をCASで照合してprivacyだけを解除し、受付と実行を最小限の完了監査記録にする。

各操作は対象指定・マニフェスト・世代・cutoff・段階・安定した操作ID・最新観測・推測不能な追跡情報を照合する。不明・欠落・別範囲・pending/remaining > 0・全実行系の停止未確認・復元排除未確認は失敗とし、カーソルを進めない。タイムアウトやACK消失時は、同じ操作IDの事後条件をアダプターで照合する。すべてのコミットを、チェックポイントのリビジョンと所有者で保護する。

独立したmoderationの世代が変わった場合は、同じprivacy request/guardSinceを確認して最新世代を採用し、途中の不存在確認をやり直す。再開意図を永続化した後は、削除・調査段階へ戻らない。再開後にBotが作った正当な新規データを、完了処理の失敗後の再試行で消してはいけない。privacyの解除では独立して有効なmoderationを維持する。すべて無効なSACチェックポイントも世代とともに保持し、TTL・削除・リセットを行わない。

完了受付にはschemaVersion・acceptedAt・deleteBy・completedAtだけを保持する。実行の完了レコードには、同じ対象指定とマニフェストの完了照合専用ダイジェストを追加し、別の対象・申請・証拠に完了結果を転用できないようにする。元の関連参照は残さないが、この推測不能なダイジェストを匿名化の完全な証明とは扱わない。チャンネル・証拠・所有者・申請ID・OAuthやインデックスへの関連・マニフェスト・証跡列を除去し、現在のOAuth・申請インデックス・証拠確保情報を同じトランザクションで消す。推測不能な完了IDの再照会では操作を実行せず、完了を返す。新しい保持期間は導入しない。外部保管した運用マニフェスト・ログ・エクスポート自体の消去は、各対象範囲の責務であり、この処理だけで匿名化済みとは主張しない。

## アダプターの実装範囲

[削除対象と起動経路](support-delete-inventory.md)で、ソースから確認できる範囲と実環境で確認すべき範囲を区別する。Manifest.Refは、信頼済みの登録処理が対象環境・プロジェクト・DB、全書き込み・起動経路、開始前状態、旧キュー・import・restoreの排除方針、スキーマの帰属、ワークフロー計画のリビジョンを確認して発行するダイジェストである。運用JSONの文字列だけから発行してはいけない。稼働中チェックポイントのカーソルを別の計画に読み替えない。計画変更時は新マニフェストとの照合で停止し、既存実行の明示的移行を別途検証する。

`FirestoreDeletionStore`は、渡されたクライアントのプロジェクトと既定DBのパスを検証する。`FirestoreDeletionEffects`は以下のソース許可リストを実装する。信頼済みRestoreGuardが、同じ操作について全実行系の停止・完了待ち、一覧の完全性、復元の排除を証明するまで書き込まない。

| scope | implemented source allowlist / limitations |
| --- | --- |
| seat-state | seats/member-seats の user-id query を専用消去。通常 Out/Block の activity/RP/Ban を生成しない |
| firestore-primary | users direct doc、work segments、activity、order、4 seat limits、legacy history の field query |
| firestore-web | channel WebAccount doc。process cache/flight/配達の確認は runtime adapter 必須 |
| firestore-oauth | channel OAuth と current receipt の support OAuth query。current verification trace は finalize まで保持。channel 未確定 callback は runtime drain 必須 |
| firestore-support-relations | current challenge / duplicate request index の消去。canonical index / current claim は finalize まで保持。他の target receipt / owned execution / proof claim があれば unknown で停止し、勝手に completed にしない |

削除では上限付きの先頭ページをトランザクションごとに取り直し、同じ所有者・リビジョン・世代を読んでから削除する。部分コミットを跨ぐオフセットによる取りこぼしを避け、空ページも再確認する。上限は101件 × 10,000ページとし、到達時は未完了とする。最終調査で空でなければ失敗とする。別チャンネル・SAC・現在の実行や証拠は消去しない。

登録されていない対象範囲は常に取得不能とする。旧マッピングの逆引き・重複UIDと古い証拠・受付の関連、Authテナントの全本人情報と発行中トークン、BQのmain/tmp/job、混合・soft-deletedバックアップとPITR、チャンネル索引のないトレンド・ベンダー・ログは、APIの保証と帰属が確定するまで実環境用アダプターを作らない。ソースの`GetAll`やクエリ0件を、全ストアの不存在と解釈しない。既存手順書の対象外である混合バックアップの一括削除を、このCLIで代行しない。

## 合成アダプターの仕様

全アダプターは合成検証用である。実接続・権限・完全な対象一覧・削除結果の再読み取りが揃うまで、実環境での使用を拒否する。

### Auth と関連レコード

`FirebaseDeletionEffects` は injected fake SDK と Firestore Emulator で試験する adapter。既存 `Operation` / `Evidence` / stable operation ID / 29 step を変更せず、`firebase-auth`、`legacy-mappings`、`firestore-oauth`、`firestore-support-relations` を一つの ownership snapshot に基づいて扱う。Registry / CLI / server の live bootstrap には接続していない。`live` manifest は拒否し、`--execute` の `LIVE_ADAPTERS_UNAVAILABLE` と全 live gate を維持する。

最初の revoke より前に、独立した `AuthOwnershipInventory` から full execution binding、complete な UID / tenant / user 作成時刻 / exclusive channel owner、legacy mapping と関連 document 全体の fingerprint を受け取る。空の UID list も明示的な complete inventory が必要。channel 文字列を現在の UID と推測したり、tenant を既定値にしたり、`firebase-uid` field 一つを ownership としたりしない。Emulator は direct legacy UID document、reverse mapping query、receipt / OAuth / challenge / request index と逆参照を照合し、欠落・共有・別 channel・上限超過を拒否する。fixture inventory の注入は実環境の完全性を証明しない。

snapshot は server-only `support-delete-auth-ownership/{executionRef}` に保存する。selector、manifest、owner、revision、generation、cutoff の binding は checkpoint の Commit / Refresh / Recover と同じ transaction で持ち越す。legacy mapping の削除と `mappingsRemoved` は原子的。再開時は mapping の不存在から旧 UID を推測せず、同じ snapshot を使用する。後続 step の snapshot 欠落や差し替えは停止条件。終端 Finalize は receipt / execution の最小化、current OAuth / index / proof claim の消去、snapshot の消去、privacy clear を同じ transaction で行う。snapshot に独立した恒久 retention / TTL / SLA を導入しない。

初回 Acquire は有効な OAuth trace を検証し、proof claim と execution を原子的に保存する。その後の同一 execution は、current receipt / proof claim / privacy fence の binding を照合し、snapshot 保存前後の current OAuth の TTL 消失を許容する。TTL 消失済みの trace は complete inventory に残っていても省かれていてもよいが、残存・再作成された OAuth は inventory と fingerprint を必須とし、shared OAuth の逆参照も検査する。missing / 別 request / 別 execution の claim は拒否する。再認証や SLA のリセットは行わない。

SDK の `GetUser` / `RevokeRefreshTokens` / `DeleteUser` interface は pinned Firebase Admin SDK の project client と tenant client が満たす。constructor、ADC、認証、実 API 呼出しは追加していない。project / tenant client と NotFound classifier は明示注入し、permission、timeout、unknown は不存在へ変換しない。SDK の success response も完了の証拠とせず、同じ UID / tenant / user 作成時刻の definitive readback で revoke または不存在を確認する。lost acknowledgement 後も同じ operation ID と snapshot で結果を照合し、UID 再利用を検出したら新しい user を消さない。

`DeleteUser(uid)` は incarnation 条件付き CAS ではない。`AuthDeletionGuard.WithGuard` は SDK の read / mutation / readback を含む callback 全体で UID 再作成・import・所有変更を排除する独立保証を必須とする。snapshot の作成時刻だけではこの保証を満たせない。guard は同じ operation の trusted fleet / restore evidence、同期 clock、全 token consumer の check-revoked-token gate も明示する。これらが欠ければ mutation を始めない。revoke の `TokensValidAfterMillis` は UTC 秒粒度なので cutoff の次の秒以上を確認する。revoke acknowledgement 自体は既発行 ID token の失効を証明しない。既存の全 token consumer、mint / delivery、再投入排除の実保証は外部 gate のまま。

関連削除は独立 inventory に含まれ、他の owned execution / proof claim を持たない receipt とその OAuth / challenge / request index に限定する。別の active execution、shared proof / OAuth、unknown relation があれば transaction 全体を止め、他の receipt を勝手に completed にしない。current receipt、current verification OAuth、canonical request index、current proof claim は Finalize まで保持する。別 channel、SAC、他 execution は削除しない。

fake SDK tests は wrong UID / tenant / incarnation、shared owner、missing inventory、guard 欠落、秒境界、timeout / lost acknowledgement / retry、UID 再利用を確認する。Emulator tests は durable snapshot と mapping 削除後の再開、reverse owner / inventory 欠落の rollback、関連閉路と他 channel isolation、owner recovery / generation fence、atomic terminal minimization、anonymous / authenticated Client の read / write deny を確認する。これらは実 UID / tenant / IAM、fleet、BQ、backup、vendor の不存在や全 live deletion 完了を証明しない。

### BigQuery

`BigQueryDeletionEffects`は、注入した`SyntheticBigQueryExecutor`だけを使い、既存の`bigquery`範囲を実装する。SDK実行器・認証情報探索・実クエリ・IAM変更・ランタイム統合・CLI初期化は追加しない。`live`は拒否し、`--execute`も引き続き`LIVE_ADAPTERS_UNAVAILABLE`を返す。合成処理の完了は、実削除の証拠にならない。

独立した完全な一覧で、実行・マニフェスト・cutoff、明示したプロジェクトとロケーション、必須の活動・注文・過去の生チャットの本テーブル、共有`tmp`、対象結果テーブル、旧load/query/copy/exportジョブのID、推測不能な外部コピー参照を固定する。既存テーブルには検証済みスキーマと実体世代のフィンガープリントを必要とする。空・欠落スキーマは不明として扱い、0件にはしない。明示的に不存在の廃止テーブルも、一覧と対象件数のインターフェースで再観測する。識別子は限定した許可リストを使い、チャンネル値は`@channel`でバインドしてSQLに埋め込まない。純粋な計画処理はSQL生成だけを行い、実行できない。データセット・テーブル全体の削除や、生チャット廃止時の一括消去は再利用しない。

一覧取得インターフェースは、固定した取得元・ジョブ・コピーの全体を列挙する必要がある。この一覧から確定的に計画する削除ジョブは別に数える。メタデータの過剰・欠落・変更・不完全性があれば実行を止める。変更前に、旧取得元ジョブの確定した終了状態と再実行の排除を必要とする。呼び出し元やタスクのキャンセル、取消ACKだけでは足りない。独立した信頼済み保護処理は、各コールバック中に一覧・実体世代の変更を排除し、削除中の全実行系停止、ジョブの実行保護、旧import/restore排除を証明する。テストデータの`true`フラグは、実環境の停止を証明しない。

サーバー専用の`support-delete-bigquery-inventory/{executionRef}`は、不変の一覧、確定的なジョブ計画の入力、削除・調査の確認結果を保存する。`support-delete-bigquery-bindings/{executionRef}`と、実行中文書の任意の不変取得マーカー`bigQueryInventoryRef`も同時にアトミック作成する。実行マーカーが残る間は、一時文書の片方または両方が失われたり置き換わったりしても、部分実行後に再取得しない。マーカーのない旧実行は読み取り可能なままにし、通常のチェックポイント更新はマーカーを保持し、完了時の置換で除去する。取得前の実行全体の復元も、独立した復元保護で引き続き禁止する。
ジョブID・設定ダイジェストは再試行・所有者復旧・独立したモデレーション変更を跨いで固定する。送信成功やタイムアウトは、同じプロジェクト・ロケーション・ジョブ・設定の確定的な検索で照合する。保留・失敗・調査時の不存在・不一致・不明なジョブはワークフローを止める。全テーブルの対象件数は最新観測で確定した0である必要があり、処理後に一覧も再検査する。

スナップショットの実行保護情報は、Auth所有権とワークフローのチェックポイントと同じFirestoreトランザクションで進める。各テーブル操作で、同じ永続的な証拠確保情報・受付・正式申請インデックスと再照合する。OAuthがTTLで消えても再証明を求めない。所有者や世代がずれた後、遅れた操作は有効な証跡を公開できない。トランザクションで外部操作までアトミックにはできないため、独立した保護処理と永続的なリモートジョブ識別は引き続き必要である。再開意図の永続化前に世代が変われば、調査確認を無効にする。完了時は、BQの両文書、Authスナップショット・受付の関連をアトミックに除去してprivacyを解除する。無効状態のSAC世代と独立したmoderationは残す。新たな恒久保持やTTLは導入しない。

外部のGCS・ベンダー・ログのコピーは、このアダプターでは削除しない。BQ削除前に再投入を封じ、それぞれの担当範囲で処置し、BQ調査前に申請に結び付いた処置結果を証明する必要がある。不明なコピーは完了を妨げ、本テーブル0件だけでは全ストアの不存在を証明しない。実環境のtime travel・バックアップ・エクスポート・ログの残存、実APIの保証、完全なジョブ・テーブル一覧、権限は技術・運用上の確認事項として残る。この合成実行器は、それらを証明したり新しい保持ポリシーの根拠にしたりしない。

状態を持つ合成テストで、main/tmp/結果テーブルの対象・対象外行、スキーマ・実体世代・一覧の変化、誤プロジェクト・ロケーション、旧保留・不明ジョブ、送信・チェックポイントのACK消失、復旧IDの安定性、古い件数・取得失敗、外部コピーの不明性、永続状態の欠落を検証する。Firestore Emulatorでは、Authとスナップショットのアトミックな移動、世代・復旧の実行保護、片側状態の消失、ロールバック・完了時消去、クライアント拒否、Auth・再開・完了前のエンジン拒否、再開後の正当なデータを保つ完了再試行を検証する。他の必須範囲は明示したテストデータか取得不能とし、暗黙に成功させない。

既存のリポジトリ検査を使う。Goテスト、対象を絞ったrace、lint、ルートのFirestore Emulatorスクリプト、MyPage仕様、文書参照検査を実行する。実BigQuery SQLの実行や実環境の不存在を検証するものではない。

### 派生データとvendor

`DerivedDeletionEffects`は、注入した一覧・保護付き永続化・状態を持つ合成実行器で既存の`derived-vendor`段階を実装する。ベンダーは呼び出さない。`live`は利用不能、CLIの`--execute`は`LIVE_ADAPTERS_UNAVAILABLE`のままにする。偽物の削除・再読み取りは、実際のOpenAI・Discord・YouTube・キャッシュの廃棄を証明しない。

対象一覧にはBFFキャッシュ、全体の作業名トレンド、OpenAIの保存済みResponses、Discordメッセージ、サービスが作成したYouTubeコピーを明示する。独立したmoderationやYouTube Ban状態はプライバシー削除の対象外とする。0件を報告する取得元も、完全性と廃棄対応を証明する必要がある。各項目は正確なオブジェクトID、実体世代のフィンガープリント、全所有チャンネル、対応する処置を持つ。単独所有はオブジェクトごと削除できる。共有物は対象部分だけを除去し、他の全所有者を保持する再読み取りが必要である。帰属欠落・一覧不明・非対応処置・実体世代変更・予期しない対象コピー・不確かな再読み取りは、その段階を止める。取得元の所有情報がない全体ランキングでは、保存スキーマにチャンネル項目がないことだけで対象データなしと推測しない。

ソース調査では、OpenAI Responsesの`Store: true`を確認したが、レスポンスIDは保存していない。`ChannelMessageSend`も返されたDiscordメッセージIDを破棄する。旧トレンド出力はチャンネル帰属のない共有例を含む。このため、実一覧と削除能力は不明のままである。推測したIDフィールドや保持期間は追加しない。合成テストデータのオブジェクトIDは偽物の内部だけに存在する。YouTubeコピーの所有権も不明であり、基盤ログは別の対象範囲で扱う。

サーバー専用スナップショットと対応情報は、実行中の不変マーカーとアトミックに取得する。所有者・リビジョン・世代・カーソルはチェックポイントとともに進め、取得後に片側を失っても再取得しない。変更IDは取得済みの操作・一覧・取得元・オブジェクト・実体世代から導出し、復旧後も維持する。タイムアウト・ACK消失後は確定的に読み取る。部分完了では同じIDを再試行し、処置済み項目も観測する。`inspect`は前進前に全項目と一覧全体を再確認する。再開意図の永続化前に世代が変わると調査確認を無効にする。完了時は一時スナップショットとマーカーを除去し、独立して有効なmoderationは残す。

Firestore Storeは、他が有効な汎用Evidenceを呼び出し元が渡しても、このスナップショットなしで派生データ削除段階を進めるチェックポイントを拒否する。一度進んだ後は、スナップショットの不存在により調査・復旧・完了を拒否する。取得前の段階と、未取得の削除カーソルでの明示復旧は可能なままとする。取得後のスナップショット・対応情報・マーカーの片側欠落や不一致では、常に再取得を拒否する。

実接続前に運用担当者は、取得元時点のチャンネル所有権、実IDとプロバイダー対応の対象処置、旧書き込み・キューの排除、正確な再読み取りの意味、他所有者の保持、復元排除を証明する必要がある。対象部分だけを安全に操作できない共有コピーは停止したままにし、一括破壊してはならない。D02の設計方針は、外部Google権限取消の検出だけのためにリフレッシュトークン保存や定期取得を追加しないことである。規約適合性と取消後の保持済みAPIデータの扱いは、引き続き検証・規約レビューが必要である。D03の公開運営者・連絡先・URLの実値は未決定であり、この合成アダプターは実削除を承認しない。

## 合成Registryと開始前の組み立て

[Registry](../../system/core/supportdelete/registry.go)は、信頼済みテストデータから渡された接続を開始前に検査する。ランタイム2件・削除11件と、既存29段階の必須操作がすべて揃うことを確認する。対象範囲の重複・未知操作・欠落・nil/typed-nilアダプター・対象やモードの不一致がない場合だけ、非公開の接続マップにコピーする。呼び出し元が登録スライスや操作を書き換えても、選択済みアダプターは変わらない。アダプター内部の可変状態を複製・停止する機能ではなく、各アダプター自身の保護・実行世代検証が必要である。

[PrepareDeletion](../../system/core/supportdelete/prepare.go)は独立した合成一覧と対象指定・マニフェストを照合し、Registry検査後だけ、明示的に渡されたStore factoryを呼ぶ。不足時はfactory・Acquire・復旧・操作を実行しない。準備の成功自体は、証拠確保や削除ではない。マニフェストのRefは信頼済みテストデータが供給し、文書や運用入力から実証跡を発行しない。`development`の`demo-`プロジェクトと`mock`/`emulator`に限り、完全な偽接続でも`live`は拒否する。SDK構築・ADC探索・実接続先選択は追加しない。注入factoryのクライアントの有効期間は呼び出し元が管理する。

返すPreparedDeletionは、対象指定・マニフェスト・接続を変更可能な形で公開しない。Runと明示Recoverを同じ結び付け済み実行に限定する。同じ組み立て結果への同時呼び出しはbusyで拒否する。このプロセス内ロックは、全実行系の停止・完了待ちの証拠ではない。Recover案だけで所有者を移さず、Storeが停止の追跡情報と永続リビジョン・世代を検査した後だけ更新する。Recoverのタイムアウト・ACK消失は未確認として返し、所有者移転済みと推測しない。明示した新所有者の同じ対応情報で別の組み立て結果を作り、正当なAcquire結果で再開可否を確認する。

RunMockの通常テストデータと停止済みworkerのテストデータは、両方ともこの処理で組み立てる。既存29段階、安定した操作ID、acceptedAtからの7日、証拠・世代・チェックポイント、再開後の正当な新規データ保護は、Engine/Storeの既存仕様を維持する。CLIのスキーマ・フラグ・報告は変更しない。`--execute`はSDK初期化やモック処理に到達せず、従来の`LIVE_ADAPTERS_UNAVAILABLE`で拒否する。実一覧の発行、運用担当者の削除権限、全実行系・実ストアのアダプターは未接続のままとする。

## オフラインCLI

`MYPAGE_ENVIRONMENT`と`GOOGLE_CLOUD_PROJECT`を明示する。プロジェクト別名の環境変数が不一致の場合や、実認証情報・実接続先の環境変数がある場合は拒否する。対象は標準入力か、権限が正確に0600の通常ファイルを読む`--manifest`で渡す。末尾のシンボリックリンク・FIFO、16KiB超、重複・大小文字違い・未知・欠落のJSONフィールド、確認内容の不一致を拒否する。報告に対象・参照・パス・元のエラーは出さない。

合成テストデータ専用のマニフェスト形式を示す。

```json
{
  "schemaVersion": 1,
  "operation": "delete",
  "target": {"environment":"development","projectID":"demo-youtube-study-space-ci","channelID":"UCsynthetic0000000000001"},
  "requestRef":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "executionRef":"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "proofRef":"2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "confirmation": {
    "operation":"delete",
    "environment":"development","projectID":"demo-youtube-study-space-ci","channelID":"UCsynthetic0000000000001",
    "requestRef":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "executionRef":"1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "proofRef":"2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}
```

```sh
cd system
MYPAGE_ENVIRONMENT=development GOOGLE_CLOUD_PROJECT=demo-youtube-study-space-ci \
  go run ./cmd/mypage-support-delete --manifest /tmp/private-synthetic-case.json
# --check-config: offline input/config check, no Store/SDK/ADC access
# --mock: fixed synthetic case only; --mock --recover: stopped-worker fixture
# --execute: always LIVE_ADAPTERS_UNAVAILABLE, no live SDK bootstrap
```

`MOCK_COMPLETED` / `mock-only` / `offline:true` / `actualExecution:false`を実削除の証跡に転用しない。モックの参照を変えても、新たな本人確認の証拠は生成されない。

## 実接続前の技術的確認と判断

実環境では、別途明示した運用として開発→本番の順に開始する。まずソースの対象一覧を、実配備・旧リビジョン・直接起動・エクスポート・復元と照合する。全ランタイムアダプターの停止・完了待ち・再開と管理者、全ストアアダプターの全件消去・再読み取り・旧ソース再投入禁止を個別に検証する。保証のない範囲を成功扱いにするオプションは追加しない。実SDK認証情報の取得・停止・失効・削除は合成検証に含めない。

実環境用アダプターの未実装、APIの停止保証、コピーの帰属、本人確認追跡情報の欠落は、技術・外部検証上の確認事項である。「ユーザー承認で成功扱いにする」判断待ちに変えてはならない。無効状態のチェックポイント保持、検証不能時の拒否、cutoff、同一実行の再開、requestRef + generation CASは承認済みの技術判断である。

追加の人間判断が必要なのは、実一覧で既存Canonの7日以内のアクセス可能コピー消去を満たせず例外・期限変更が必要な場合、帰属を証明できない共有バックアップ・ベンダーコピーの破壊が他者データの扱いを変える場合、新しい保持期間や利用制限を提案する場合である。現在はそのような変更を実装しておらず、実環境では引き続き検証不能時に拒否する。

## 検証

外部処理を持たないワークフローは、全29段階の操作前後の失敗、チェックポイントACK消失、同一IDの再試行、全必須範囲との対応、所有者復旧、世代と新規利用の区切りを確定的なテストデータで検証する。Firestore Emulatorでは、実ソースアダプターの証拠・保護・確保の原子性、並行する初回確保、追跡情報欠落時のロールバック、SLA維持、所有者復旧、世代・申請CAS、moderation保持、関連参照の消去、1002件のページ分割と他チャンネルの分離、クライアントルールの拒否を確認する。実Auth・BQ・バックアップ・ベンダーの不存在を証明するものではない。

```sh
cd system
go test -shuffle=on ./...
go test -race -shuffle=on ./core/supportdelete ./core/mypage ./cmd/mypage-support-delete
golangci-lint run --timeout=5m --config=.golangci.yml
I18N_BASELINE=ja go generate ./...
# repository root
bash .github/scripts/run-firestore-integration-tests.sh
python3 .github/scripts/check-doc-references.py
```
