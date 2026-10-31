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

## 適用前の準備

DEV／本番とも、既定値の`monitor_auth_registration = null`ではリソースも認証設定も変更しません。`normal.tfvars.json`に設定を追加する変更をレビューしてから有効化してください。ここにソースを追加しただけでは、稼働中の新規登録は制限されません。

1. 対象プロジェクトのFirebase Authentication with Identity Platformが有効になっていること、課金・利用条件、必要なAPIを確認します。既存Auth設定を取り込む構成であり、プロジェクトやAuthを新規作成・初期化しません。API有効化は既存のAPI管理元で行い、所有権を重複させません。必要なAPIはIdentity Toolkit、Cloud Functions、Cloud Build、Cloud Run、Artifact Registryです。
2. 既存Auth設定とTerraform Stateの所有者を確認します。他のStateに同じAuth設定がある場合は、先に所有権を整理してください。既存の`beforeCreate`は上書きしません。既存`beforeSignIn`のURLがある場合は、下記の入力に正確に指定します。通常Planの事前確認は、別の`beforeCreate`、未対応トリガー、`beforeSignIn`の不一致を拒否します。
3. 既存の通常Terraform Plan／Apply用SAと環境別Stateを準備します。認証付きの通常運用は[`infra/gcp/README.md`](../../README.md)に従います。マイページ専用Stateには追加しません。
4. 管理端末で専用SAと追加権限の予定を確認します。以下のスクリプトは、既定では予定を表示するだけです。

```sh
python3 infra/gcp/scripts/bootstrap_monitor_auth.py --target dev
```

レビューした対象環境への権限準備を実行するときだけ、同じコマンドに`--apply`を追加します。本番は`--target prod`を明示します。この操作は管理者の短期認証を使い、長期鍵を作りません。既存のWIF・AWS backend・GitHub Environmentの保護を変更しません。

追加する権限は、Auth構成の参照／更新、認証用ソースバケットだけの読み取り／新規オブジェクト作成、対象関数だけのInvokerポリシー更新です。ビルド用SAにはコードの読み取り・専用リポジトリへの書き込み・ビルドログ出力だけを付与します。実行用SAにFirestore・Authユーザー管理・Secret読み取り・トークン署名の権限は付与しません。既存実行用SAにプロジェクト権限がある場合や、同名カスタムロールの内容が異なる場合は停止します。

通常の構築用SAへ`firebaseauth.configs.getHashConfig`を付与しないでください。TerraformのAuthリソースは公開APIキーなどをStateに保存するため、既存どおりStateと生Planを非公開に保ちます。関数の最小インスタンス数は0、最大は2です。起動・ビルド・ログ・保管に利用料が発生し得ます。

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
