# Menu Image Generator

Firestoreの`menu`コレクションからメニュー表画像（JPEG）を生成するCLIツールです。

## 機能

- Firestoreからメニューアイテムを取得（code順でソート）
- React + インラインスタイルでメニュー表をレンダリング
- Puppeteerで2048x2048のJPEG画像を生成
- アイテム数が多い場合は複数ページに分割（1ページ最大12アイテム）

## セットアップ

### 1. 依存関係のインストール

```bash
npm install
```

### 2. Firebase認証の設定

1. Firebase Consoleからサービスアカウントの秘密鍵JSONファイルをダウンロード
2. `env.example`をコピーして`.env`を作成
3. `GOOGLE_APPLICATION_CREDENTIALS`にJSONファイルのパスを設定

```bash
cp env.example .env
```

```env
GOOGLE_APPLICATION_CREDENTIALS=./serviceAccount.json
```

## 使い方

```bash
npm run generate
```

出力先: `output/menu_1.jpg`（アイテム数に応じて`menu_2.jpg`...）
