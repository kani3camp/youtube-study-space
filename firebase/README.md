# Firebase / Firestore

このディレクトリでは、Firestore Security Rules・インデックス・エミュレーターの設定をリポジトリで管理しています。これらを既存プロジェクトの定義として扱ってください。設定の上書きや再生成を避けるため、通常のセットアップで `firebase init firestore` を実行しないでください。

## Firebase Authenticationの管理元

メール・パスワード認証の有効化は、[GCP Terraformの認証用モジュール](../infra/gcp/modules/monitor-auth/README.md)で管理します。`beforeCreate`による新規登録制限は廃止し、公開登録と既存アカウントへのパスワード追加を許可します。`firebase.json`には同じAuth設定の配備設定を追加せず、Firebase CLIとTerraformによる二重管理を避けてください。

このディレクトリは引き続きFirestore Rules・インデックス・エミュレーター設定の管理元です。モニター専用ユーザーはFirebaseコンソール／Admin SDKで作成し、許可リストを別途準備します。認証用モジュールは環境ごとの明示的な有効化と適用が必要で、コードのマージだけでは実環境の認証設定は変わりません。

## テストで確認する範囲

以下の検証は、それぞれ異なる仕様や制約を確認します。互いに代用することはできません。

### 1. 通常のGoテスト

```sh
cd system
go test -shuffle=on -v ./...
```

`integration` ビルドタグを持つテストは含まれません。

### 2. Firestore Emulatorを使う統合テスト

リポジトリのルートから実行してください。

```sh
npm install --global firebase-tools@15.25.1
bash .github/scripts/run-firestore-integration-tests.sh
```

Node.js、Java 21以降、CIで固定しているバージョンのFirebase CLIが必要です。スクリプトはFirestore Emulatorを起動してGoの統合テストを実行します。実際のFirebaseプロジェクトや本番用の認証情報は不要です。

スクリプトは、このディレクトリ専用の依存関係をインストールし、`@firebase/rules-unit-testing` で `firestore.rules.test.mjs` を実行してから、Goの統合テストを実行します。Go側では、Firestoreへのシリアライズ・トランザクション・リポジトリおよびアプリケーションの動作を、エミュレーターで検証します。Rulesのテストでは別途、ブラウザのClient SDKによるアクセスが許可・拒否されることを検証します。

### 3. Client SDK / Security Rulesの動作

`firestore.rules` を変更するときは、サーバー側のGoの動作と、ブラウザ・クライアントからのアクセスを分けて確認してください。特に `config/*` や `youtube-monitor` が利用するコレクションなど、変更するアクセス制御の対象ごとに、意図した読み書きが許可・拒否されることを確認します。

サーバー用の認証情報を使うGoテストは、Security Rulesによるアクセス制御を迂回できます。Goのエミュレーター統合テストが通っただけで、クライアントのアクセス制御を検証できたと判断しないでください。

モニターの読み取りには、Firebaseへのログインと、ログインしたUIDの `_monitor-readers/{uid}` 文書の `enabled: true` が必要です。カスタムクレームは使用しません。クライアント向けのRulesでは、この許可リストの参照・変更を認めません。許可リスト文書は、信頼できるAdmin / サーバー操作で準備してください。段階的な導入と、アクセスを閉じた状態を維持するロールバックの手順は [`../youtube-monitor/README.md`](../youtube-monitor/README.md) を参照してください。

## 既存プロジェクト：配備前の確認

実際のFirebaseプロジェクトを確認・変更する必要がある場合だけ、認証を行ってください。

```sh
firebase login
firebase projects:list
firebase use <project-id>
```

配備前には、選択中のプロジェクトを明示的に確認します。

```sh
firebase use
```

適用前に、リポジトリで管理している設定を確認してください。

```sh
cat firebase/firestore.rules
cat firebase/firestore.indexes.json
cat firebase/firebase.json
```

Firebase CLIには、Firestore Rules・インデックスを対象としたCDKのような汎用の `diff` はありません。ローカルのソースレビューと上記のエミュレーター検証を行い、実環境の検証が必要な場合はFirebase / GCPで対象プロジェクトの状態を確認してください。

## 対象を絞った配備

配備は外部プロジェクトの状態を変更するため、対象環境への操作として承認が必要です。`firebase deploy` で全体を配備するのではなく、必要な対象に絞ってください。

```sh
cd firebase
firebase deploy --only firestore:rules --project <project-id>
firebase deploy --only firestore:indexes --project <project-id>
```

Rulesとインデックスの両方を意図的に変更する場合だけ、まとめて配備します。

```sh
firebase deploy --only firestore --project <project-id>
```

配備後は、対象環境でクライアントから見た動作が想定どおりであることを確認してください。Rulesの構文が正しいかを調べるためだけに配備せず、まずローカルとエミュレーターで検証します。

## 独立したFirebaseプロジェクトの新規作成

初期化コマンドを使うのは、意図的に別のFirebaseプロジェクトを新規作成する場合だけです。既存環境の通常作業には含めません。新規作成用の手順が必要になった場合も、管理済みの設定を誤って上書きしないよう、既存環境の作業手順とは分けて記載してください。
