# モニター用アカウントの新規登録制限

メール・パスワードでの一般利用者によるアカウント作成だけを、Firebase Authenticationの`beforeCreate`で拒否します。Firebaseコンソール／Admin SDKによる管理者作成と、作成済みユーザーのパスワードログインは維持します。マイページはGoogle／YouTubeでの本人確認後にカスタム認証を使うため、この登録チェックを起動しません。

このモジュールを設定・関数の正本とします。Firebase CLIの`deploy --only auth`や`deploy --only functions`では、同じ認証設定・関数を配備しないでください。Firestore Rulesとインデックスは引き続き[`firebase/`](../../../../firebase/README.md)で管理します。

## 管理する範囲

- メール・パスワード認証の有効化と、メールログインへのパスワード必須設定
- 登録拒否の関数、専用の非公開ソース保管バケット、専用のビルドイメージ保管先
- 対象関数だけの公開呼び出し権限。HTTPイベントの署名検証はFirebase公式SDKが担当
- `beforeCreate`への関数の登録。既存の`beforeSignIn`がある場合は、そのURLを維持

GoogleのIdP設定は管理対象に含めません。既存の許可ドメイン・匿名認証・電話認証・MFA・クォータ・ユーザー操作権限なども、このモジュールで変更しません。`disabledUserSignup`によるプロジェクト全体の登録禁止は使用しません。既存のメールリンク認証を継続したい環境には、そのまま適用しないでください。この構成ではメールログインにパスワードを必須にします。

関数のコードは[`function/`](function/)にあります。SDKはロックファイルで固定し、Cloud Buildが依存をインストールします。配備ZIPには明示したソースと依存定義の4ファイルだけを含め、`node_modules`、テスト、認証情報、一時ファイルを含めません。同じコードからは同じZIPを作り、SHA-256をオブジェクト名に使用します。旧オブジェクトは保持し、既知のコードへ戻せるようにします。

## 現行の制限範囲とパスワードリンクの未解決事項

この構成が拒否するのは、新しいユーザーを作成するメール・パスワード登録です。既存アカウントへパスワード方式を追加する処理は `beforeCreate` を通らないため、この関数だけでは拒否できません。Firebaseのアカウントリンク設定やメールアドレス列挙対策を変更すれば全面的に禁止できる、とも扱わないでください。

公式仕様でも、[アカウントリンク設定は手動リンクを禁止しない](https://docs.cloud.google.com/identity-platform/docs/link-accounts)こと、[メールアドレス列挙対策を有効にしても `accounts:signUp` にIDトークンを渡すリンク経路がある](https://docs.cloud.google.com/identity-platform/docs/admin/email-enumeration-protection)ことが明記されています。

Auth Emulatorでは、マイページと同じCustom Authenticationで初回作成したUIDに対し、IDトークン付きの `accounts:signUp` と `accounts:update` の両方でパスワード追加・パスワードログインが成功し、関数の呼び出し回数が増えないことを再現しています。このテストは現行の穴を記録するための再現テストであり、リンク許可を将来の受け入れ要件にするものではありません。

DEV（`test-youtube-study-space`）でも、パスワードを持たない検証用の既存アカウントに対する `accounts:signUp` のリンクと、その後のパスワードログインが成功しました。メールアドレス列挙対策は有効で、旧 `accounts:update` 経路は `OPERATION_NOT_ALLOWED` で拒否されました。Auth構成は変更せず、一時ユーザー2件を削除して残存0件を確認しています。DEVでは既存権限でCustom Tokenを署名できなかったため、管理者作成した検証用アカウントからパスワードを解除してリンク前の状態を作りました。DEVでCustom Authenticationの経路まで検証済みとは扱いません。

再現用のDEVコマンドは次のとおりです。固定DEVプロジェクトとWeb APIキーの対象を照合し、専用の一時UIDだけを作成・リンク・削除します。プロジェクトのAuth設定・IAM・Firestore・既存ユーザーは変更しません。`gcloud` の短期認証と、リポジトリの `youtube-monitor/.env` に設定されたDEVの公開クライアント設定を使用します。トークン・パスワード・生のAPI応答は表示しません。削除に失敗した場合だけ、後片付け用に生成した検証UIDを表示します。

```sh
node infra/gcp/modules/monitor-auth-registration/function/dev-link-probe.mjs --target dev
```

厳密に「一般ユーザーによるパスワード追加自体を禁止」する要件と、「追加は許容してパスワードでの再ログインを運用者に限定」する要件は別です。現行構成はどちらの完全な実装でもありません。前者では認証領域の分離等を含めた設計変更、後者でもリンク時に発行されるトークンや既存セッションを考慮した利用境界の定義が必要です。要件と移行方針を確定するまで、この問題を修正済みとして扱わないでください。

## 適用前の準備

変数の既定値`monitor_auth_registration = null`ではリソースも認証設定も変更しません。DEVは`normal.tfvars.json`で明示的に有効化し、本番の入力は無効のまま維持します。設定変更とTerraform Applyの結果は対応PR／Actionsを参照してください。ソースや入力を追加しただけでは、稼働中の新規登録は制限されません。

1. 対象プロジェクトのFirebase Authentication with Identity Platformが有効になっていること、課金・利用条件、必要なAPIを確認します。既存Auth設定を取り込む構成であり、プロジェクトやAuthを新規作成・初期化しません。API有効化は既存のAPI管理元で行い、所有権を重複させません。必要なAPIはIdentity Toolkit、Cloud Functions、Cloud Build、Cloud Run、Artifact Registryです。
2. 既存Auth設定とTerraform Stateの所有者を確認します。他のStateに同じAuth設定がある場合は、先に所有権を整理してください。既存の`beforeCreate`は上書きしません。既存`beforeSignIn`のURLがある場合は、下記の入力に正確に指定します。通常Planの事前確認は、別の`beforeCreate`、未対応トリガー、`beforeSignIn`の不一致を拒否します。
3. 既存の通常Terraform Plan／Apply用SAと環境別Stateを準備します。認証付きの通常運用は[`infra/gcp/README.md`](../../README.md)に従います。マイページ専用Stateには追加しません。
4. 管理端末で専用SAと追加権限の予定を確認します。以下のスクリプトは、既定では予定を表示するだけです。

```sh
python3 infra/gcp/scripts/bootstrap_monitor_auth.py --target dev
```

レビューした対象環境への権限準備を実行するときだけ、同じコマンドに`--apply`を追加します。本番は`--target prod`を明示します。この操作は管理者の短期認証を使い、長期鍵を作りません。既存のWIF・AWS backend・GitHub Environmentの保護を変更しません。

追加する初期権限は、Auth構成の参照／更新、認証用ソースバケットだけの読み取り／新規オブジェクト作成、ビルドログ出力です。Cloud Run／Artifact Registryのサービス単位IAM管理は、既存の通常Apply用権限カタログを使います。専用Artifact Registryのビルド用書き込み権限はTerraformがリポジトリへ直接付与し、プロジェクトの`resource.name`条件で代替しません。Cloud FunctionsがZIPを複製する`gcf-v2-sources-<対象プロジェクト番号>-<region>`についても、この関数の`monitor-password-signup-guard/`配下のオブジェクトだけを読み取り可能にします。プロジェクト番号は実行時に確認し、別関数や別環境のソースへ権限を広げません。実行用SAにFirestore・Authユーザー管理・Secret読み取り・トークン署名の権限は付与しません。既存実行用SAにプロジェクト権限がある場合や、同名カスタムロールの内容が異なる場合は停止します。

通常の構築用SAへ`firebaseauth.configs.getHashConfig`を付与しないでください。TerraformのAuthリソースは公開APIキーなどをStateに保存するため、既存どおりStateと生Planを非公開に保ちます。関数の最小インスタンス数は0、最大は2です。起動・ビルド・ログ・保管に利用料が発生し得ます。

### 秘密値を出力しない構成確認

管理者権限でAuth Config全体や`signIn`全体を表示すると、パスワードハッシュ用の秘密パラメーターも含まれます。APIの取得項目を必要な末端フィールドに限定し、応答やHTTPエラー本文をそのままログ・チャット・PRへ出力しないでください。通常Planの事前確認も、認証種別とトリガーだけを取得します。

DEVの構成確認には、短期認証を内部で取得し、非秘密項目だけを表示する次の読み取り専用コマンドを使えます。本番の指定は受け付けません。

```sh
python3 -B infra/gcp/scripts/inspect_dev_monitor_auth.py --target dev
```

ハッシュ用の秘密値は、IDトークンの署名鍵やログイン用トークンではありません。ただし、ユーザー別のパスワードハッシュ・ソルトと組み合わせるとパスワード解析に使えるため、秘密情報として扱います。[Auth Config API](https://docs.cloud.google.com/identity-platform/docs/reference/rest/v2/Config)の`hashConfig`は読み取り専用です。露出時はパスワード方式の利用状況、ユーザー別ハッシュの流出、作業ファイル・State・公開ログへの残存を確認し、プロジェクト鍵の更新方法はFirebaseサポートへ確認してください。ユーザーのパスワード変更や無関係なAPIキーの更新を、この鍵のローテーションと扱わないでください。

## 有効化と配備

DEVの例です。実行用SAとビルド用SAは分離し、両方とも対象プロジェクト内のものを使用します。値は`infra/gcp/environments/dev/normal.tfvars.json`へレビュー対象の差分として追加します。

```json
{
  "monitor_auth_registration": {
    "runtime_service_account_email": "monitor-auth-runtime@test-youtube-study-space.iam.gserviceaccount.com",
    "build_service_account_email": "monitor-auth-build@test-youtube-study-space.iam.gserviceaccount.com",
    "preserved_before_sign_in_uri": ""
  }
}
```

既存`beforeSignIn`がある場合、空文字ではなく既存のHTTPS URLを指定してください。メール・パスワード認証と登録拒否トリガーは、同じAuth構成更新で適用します。トリガーの登録は、関数の配備と呼び出し権限の設定後に行います。

既存の認証付き通常Plan／Apply経路を使います。通常実行器が、レビュー済みcommitのソースを私用作業ディレクトリへコピーし、ZIPを生成します。設定が有効なときだけ既存Auth構成の確認と取り込みを許可します。取り込み先は対象プロジェクトの`module.monitor_auth_registration[0].google_identity_platform_config.auth`だけです。それ以外の取り込みは従来どおり拒否します。Plan／ApplyのState・commit・入力・保存Planの整合検証も維持します。

初回は`import`ブロックで既存Auth構成を採用し、既存のドメインや他の認証設定を維持していることをPlanで確認します。Planの不一致、取り込み権限不足、API未有効化などを理由に保護を緩めないでください。手動のローカルTerraform実行で検証する場合も、先に次のコマンドでZIPを生成します。

```sh
python3 infra/gcp/scripts/package_monitor_auth.py
```

## 検証

ソースのCIでは、関数のテスト、Auth Emulator、TerraformのモックPlanと既存の通常実行契約を検証します。

```sh
bash infra/gcp/scripts/test_monitor_auth.sh
python3 -B infra/gcp/tests/test_monitor_auth_registration.py
python3 -B infra/gcp/scripts/test_monitor_auth_terraform.py
```

Auth Emulatorでは、公開APIからのメール・パスワード登録が保存前に拒否されること、管理者からの作成、作成済みユーザーのログイン、カスタム認証での初回ユーザー作成を確認します。署名を行わないEmulatorとの接続テストだけで、SDKの署名検証を省略する設定をテストプロセスへ渡します。本番用ZIPやTerraformの関数環境変数には、その設定を含めません。別の単体テストでは、不正な外部リクエストを成功させないことも確認します。

ソース検証と実環境での合格は分けて記録します。DEVへの適用後は、公開の登録APIが拒否されること、Firebaseコンソールで作成したモニターアカウントのログイン、マイページの未登録ユーザーによる初回登録、既存ユーザーの再ログインを確認します。その証拠を記録してから本番へ進めてください。

## 復旧

不具合時は、`monitor_auth_registration`を`null`へ戻してリソースを削除せず、有効なまま関数コードを修正して再Plan／Applyしてください。既知のcommitの関数コードを戻して配備する方法も使えます。関数とソースバケットは削除保護し、トリガーだけが残った状態で関数を削除しないようにします。一般のメール登録を許可する変更は別途レビューし、障害対応で暗黙に許可しないでください。

初期準備用のSA・追加権限は`bootstrap_monitor_auth.py`で管理者が準備し、通常の関数リリースでは変更しません。これらの削除や権限縮小も、稼働中の配備・実行への影響を確認して別途行います。

## 仕様の確認先

- [Firebaseの認証ブロッキング関数](https://firebase.google.com/docs/functions/auth-blocking-events)：カスタム認証が対象外であることと、イベント署名の扱い
- [Firebase開発チームの説明](https://github.com/firebase/firebase-tools/issues/5299)：コンソールの管理者作成では`beforeCreate`が起動しないこと
- [固定Google providerのAuth構成](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/website/docs/r/identity_platform_config.html.markdown)
