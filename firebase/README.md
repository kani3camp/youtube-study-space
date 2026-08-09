## Firestore Emulator統合テスト

ローカルで統合テストを実行するには、Node.js、Java 21以上、固定版のFirebase CLIが必要です。

```bash
npm install -g firebase-tools@15.25.1
bash .github/scripts/run-firestore-integration-tests.sh
```

このコマンドはFirestore Emulatorだけを起動し、`-tags=integration` を付けたGoテストを実行します。実在するFirebaseプロジェクトや認証情報は不要です。各テスト開始前にEmulator上のテストデータを削除します。通常の `cd system && go test -shuffle=on -v ./...` では、integrationタグ付きテストは実行されません。

## 前提条件
- Node.jsがインストールされていること
- npmがインストールされていること
- Firebaseプロジェクトが作成されていること

## 手順
### Firebase CLIをインストールする
```bash
npm install -g firebase-tools@15.25.1
```

### Googleアカウントにログインする
```bash
firebase login
```

### Firebaseプロジェクトを確認する
```bash
firebase projects:list
```

### 使用するプロジェクトに切り替える
```bash
firebase use <project-id>
```

### Firestoreについてディレクトリを初期化する
```bash
firebase init firestore
```

### Firebaseプロジェクトにセキュリティルールをデプロイする
```bash
firebase deploy
```
