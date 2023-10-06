## 前提条件
- Node.jsがインストールされていること
- npmがインストールされていること
- Firebaseプロジェクトが作成されていること

## 手順
### Firebase CLIをインストールする
```bash
npm install -g firebase-tools
```

### Googleアカウントにログインする
```bash
firebase login
```

### Firestoreについてディレクトリを初期化する
```bash
firebase init firestore
```

### Firebaseプロジェクトにセキュリティルールをデプロイする
```bash
firebase deploy
```
