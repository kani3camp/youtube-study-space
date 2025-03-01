# YouTube Study Space リファクタリングメモ

このドキュメントは、YouTube Study Spaceアプリケーションのコードベースに対するリファクタリング提案をまとめたものです。後で参照しやすいように、各リファクタリングポイントの詳細と具体的な改善案を記載しています。

## 目次

1. [エラーハンドリングの改善](#1-エラーハンドリングの改善)
2. [リトライロジックの抽象化](#2-リトライロジックの抽象化)
3. [長い関数の分割](#3-長い関数の分割)
4. [トランザクション処理の改善](#4-トランザクション処理の改善)
5. [マジックナンバー・マジック文字列の定数化](#5-マジックナンバーマジック文字列の定数化)
6. [ロギングの統一](#6-ロギングの統一)
7. [コンテキスト伝播の改善](#7-コンテキスト伝播の改善)
8. [テスト容易性の向上](#8-テスト容易性の向上)
9. [コメントの改善](#9-コメントの改善)
10. [並行処理の改善](#10-並行処理の改善)
11. [実装の優先順位](#実装の優先順位)
12. [フィールド名の統一と整合性確保](#12-フィールド名の統一と整合性確保)

## 1. エラーハンドリングの改善

### 現状の問題点
- エラーラッピングのパターンが一貫していない（`fmt.Errorf("in X(): %w", err)` と `errors.New()` の混在）
- 同じようなエラーチェックとリトライロジックが複数の場所で繰り返されている
- エラーメッセージの形式が統一されていない

### 改善案
- エラーラッピングヘルパー関数の導入
```go
func wrapError(operation string, err error) error {
    return fmt.Errorf("in %s: %w", operation, err)
}
```

- カスタムエラー型の活用拡大
```go
// core/studyspaceerror/error.go を拡張
type ErrorType string

const (
    ErrTypeNotFound ErrorType = "not_found"
    ErrTypePermission ErrorType = "permission"
    ErrTypeValidation ErrorType = "validation"
    // 他のエラータイプ
)

type Error struct {
    Type ErrorType
    Message string
    Err error
}

func (e *Error) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
    }
    return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *Error) Unwrap() error {
    return e.Err
}

func NewError(errType ErrorType, message string, err error) *Error {
    return &Error{
        Type: errType,
        Message: message,
        Err: err,
    }
}
```

### 対象ファイル
- `core/system.go`
- `core/youtubebot/live_chat.go`
- `core/repository/firestore_controller.go`
- その他エラーハンドリングを含むファイル

## 2. リトライロジックの抽象化

### 現状の問題点
- YouTube API呼び出しのリトライロジックが複数の場所で重複している
- `live_chat.go` の `ListMessages`, `PostMessage`, `BanUser` などで同様のパターンが繰り返されている

### 改善案
- リトライロジックを抽象化したヘルパー関数の導入

```go
// core/utils/retry.go
func WithRetry(operation string, maxRetries int, f func() error) error {
    var err error
    for i := 0; i < maxRetries; i++ {
        if i > 0 {
            slog.Info(fmt.Sprintf("Retrying %s (attempt %d/%d)", operation, i+1, maxRetries))
        }
        
        if err = f(); err == nil {
            return nil
        }
        
        // エラー分析とリトライ判断
        var googleErr *googleapi.Error
        if errors.As(err, &googleErr) {
            if googleErr.Code == 400 || googleErr.Code == 403 || googleErr.Code == 404 {
                // 特定のエラーコードの場合の処理
                slog.Warn(fmt.Sprintf("API error in %s: %v", operation, err))
                continue
            }
        }
        
        // その他のエラーはそのまま返す
        return fmt.Errorf("in %s: %w", operation, err)
    }
    return fmt.Errorf("max retries exceeded in %s: %w", operation, err)
}
```

- YouTube API呼び出しの改善例（`live_chat.go`）:

```go
func (b *YoutubeLiveChatBot) ListMessages(ctx context.Context, nextPageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
    var response *youtube.LiveChatMessageListResponse
    
    err := utils.WithRetry("ListMessages", 2, func() error {
        liveChatMessageService := youtube.NewLiveChatMessagesService(b.BotYoutubeService)
        part := []string{"snippet", "authorDetails"}
        
        listCall := liveChatMessageService.List(b.LiveChatId, part)
        if nextPageToken != "" {
            listCall = listCall.PageToken(nextPageToken)
        }
        
        var err error
        response, err = listCall.Do()
        if err != nil {
            // LiveChatIdが変わっている可能性がある場合
            var googleErr *googleapi.Error
            if errors.As(err, &googleErr) && (googleErr.Code == 400 || googleErr.Code == 403 || googleErr.Code == 404) {
                if refreshErr := b.refreshLiveChatId(ctx); refreshErr != nil {
                    return fmt.Errorf("failed to refresh live chat ID: %w", refreshErr)
                }
                // LiveChatIdを更新したので再試行
                return err // 再試行のためにエラーを返す
            }
        }
        return err
    })
    
    if err != nil {
        return nil, "", 0, err
    }
    
    return response.Items, response.NextPageToken, int(response.PollingIntervalMillis), nil
}
```

### 対象ファイル
- `core/youtubebot/live_chat.go`
- 新規ファイル: `core/utils/retry.go`

## 3. 長い関数の分割

### 現状の問題点
- `system.go` の一部の関数（特に `Command`, `AdjustMaxSeats` など）が長すぎる
- 単一責任の原則に反している関数がある

### 改善案
- `AdjustMaxSeats` 関数を分割する例:

```go
func (s *System) AdjustMaxSeats(ctx context.Context) error {
    slog.Info(utils.NameOf(s.AdjustMaxSeats))
    
    constants, err := s.Repository.ReadSystemConstantsConfig(ctx, nil)
    if err != nil {
        return fmt.Errorf("in ReadSystemConstantsConfig(): %w", err)
    }
    
    // 一般席の調整
    if err := s.adjustGeneralSeats(ctx, constants); err != nil {
        return fmt.Errorf("in adjustGeneralSeats(): %w", err)
    }
    
    // メンバー席の調整
    if err := s.adjustMemberSeats(ctx, constants); err != nil {
        return fmt.Errorf("in adjustMemberSeats(): %w", err)
    }
    
    return nil
}

func (s *System) adjustGeneralSeats(ctx context.Context, constants repository.ConstantsConfigDoc) error {
    if constants.DesiredMaxSeats > constants.MaxSeats {
        // 一般席を増やす処理
        return s.increaseGeneralSeats(ctx, constants)
    } else if constants.DesiredMaxSeats < constants.MaxSeats {
        // 一般席を減らす処理
        return s.decreaseGeneralSeats(ctx, constants)
    }
    return nil
}

func (s *System) adjustMemberSeats(ctx context.Context, constants repository.ConstantsConfigDoc) error {
    if constants.DesiredMemberMaxSeats > constants.MemberMaxSeats {
        // メンバー席を増やす処理
        return s.increaseMemberSeats(ctx, constants)
    } else if constants.DesiredMemberMaxSeats < constants.MemberMaxSeats {
        // メンバー席を減らす処理
        return s.decreaseMemberSeats(ctx, constants)
    }
    return nil
}

// 以下、各処理の実装...
```

### 対象ファイル
- `core/system.go` - 特に以下の関数:
  - `AdjustMaxSeats`
  - `Command`
  - `CheckLongTimeSitting`
  - `OrganizeDB`

## 4. トランザクション処理の改善

### 現状の問題点
- トランザクション処理のパターンが繰り返されている
- エラーハンドリングが複雑になっている

### 改善案
- トランザクション処理を抽象化したヘルパー関数の導入

```go
// core/utils/transaction.go
func RunTransaction[T any](ctx context.Context, s *core.System, f func(context.Context, *firestore.Transaction) (T, error)) (T, error) {
    var result T
    var resultErr error
    
    txErr := s.Repository.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
        var err error
        result, err = f(ctx, tx)
        if err != nil {
            return err
        }
        return nil
    })
    
    if txErr != nil {
        resultErr = fmt.Errorf("transaction failed: %w", txErr)
    }
    
    return result, resultErr
}
```

### 対象ファイル
- 新規ファイル: `core/utils/transaction.go`
- `core/system.go` - トランザクションを使用している関数

## 5. マジックナンバー・マジック文字列の定数化

### 現状の問題点
- コード内にハードコードされた数値や文字列がある
- 特に `live_chat.go` のリトライ回数やエラーコードなど

### 改善案
- 意味のある定数名への置き換え

```go
// core/youtubebot/constants.go
const (
    // API関連
    MaxRetryAttempts = 3
    
    // エラーコード
    ErrorCodeBadRequest = 400
    ErrorCodeForbidden = 403
    ErrorCodeNotFound = 404
    ErrorCodeServerError = 500
    
    // メッセージ関連
    MaxMessageLength = 200
    
    // その他の定数...
)
```

### 対象ファイル
- 新規ファイル: `core/youtubebot/constants.go`
- `core/youtubebot/live_chat.go`
- `core/repository/constants.go` - 既存の定数ファイルの整理

## 6. ロギングの統一

### 現状の問題点
- ロギングレベルの使い分けが一貫していない
- エラーログとデバッグログの区別が曖昧な箇所がある

### 改善案
- 構造化ロギングの一貫した活用
- ロギングヘルパー関数の導入

```go
// core/utils/logging.go
func LogInfo(ctx context.Context, message string, args ...any) {
    slog.InfoContext(ctx, message, args...)
}

func LogError(ctx context.Context, err error, message string, args ...any) {
    args = append(args, slog.Any("error", err))
    slog.ErrorContext(ctx, message, args...)
}

func LogWarn(ctx context.Context, message string, args ...any) {
    slog.WarnContext(ctx, message, args...)
}

func LogDebug(ctx context.Context, message string, args ...any) {
    slog.DebugContext(ctx, message, args...)
}
```

### 対象ファイル
- 新規ファイル: `core/utils/logging.go`
- すべてのログ出力を含むファイル

## 7. コンテキスト伝播の改善

### 現状の問題点
- コンテキストの扱いが一貫していない
- 一部の関数でコンテキストが引数として渡されていない

### 改善案
- すべての外部APIコールにコンテキストを渡す
- コンテキストのタイムアウト設定の統一

```go
// 例: MessageToLiveChat関数の改善
func (s *System) MessageToLiveChat(ctx context.Context, message string) {
    if err := s.LiveChatBot.PostMessage(ctx, message); err != nil {
        s.MessageToOwnerWithError(ctx, "failed to send live chat message \""+message+"\"", err)
    }
}

// 例: MessageToOwner関数の改善
func (s *System) MessageToOwner(ctx context.Context, message string) {
    if err := s.alertOwnerBot.SendMessage(ctx, message); err != nil {
        slog.ErrorContext(ctx, "failed to send message to owner", "error", err)
    }
}
```

### 実装状況
- [x] `MessageToLiveChat` 関数にコンテキストパラメータを追加
- [x] `MessageToOwner` 関数にコンテキストパラメータを追加
- [x] `MessageToOwnerWithError` 関数にコンテキストパラメータを追加
- [x] `MessageToModerators` 関数にコンテキストパラメータを追加
- [x] `LogToModerators` 関数にコンテキストパラメータを追加
- [x] `DiscordBot.SendMessage` 関数にコンテキストパラメータを追加
- [x] `DiscordBot.SendMessageWithError` 関数にコンテキストパラメータを追加
- [x] `MessageBot` インターフェースにコンテキストパラメータを追加
- [x] `LiveStreamChecker.Check` 内のSendMessage呼び出しにコンテキストパラメータを追加
- [x] `batch.go` 内のメッセージ関連関数呼び出しにコンテキストパラメータを追加
- [ ] その他の外部APIコールにコンテキストを追加

### 対象ファイル
- `core/system.go`
- `core/youtubebot/live_chat.go`
- `core/moderatorbot/discord.go`

## 8. テスト容易性の向上

### 現状の問題点
- テスト容易性を考慮したインターフェース設計が不十分
- モックの活用が限定的

### 改善案
- インターフェースの拡充
- 依存性注入パターンの一貫した適用

```go
// core/system.go の改善例
func NewSystem(ctx context.Context, interactive bool, clientOption option.ClientOption, 
    repoFactory func(ctx context.Context, clientOption option.ClientOption) (repository.Repository, error),
    liveChatBotFactory func(string, repository.Repository, context.Context) (youtubebot.LiveChatBot, error),
    discordBotFactory func(string, string) (moderatorbot.MessageBot, error)) (*System, error) {
    
    // 各コンポーネントの初期化...
    
    return &System{
        // フィールドの初期化...
    }, nil
}
```

### 対象ファイル
- `core/system.go`
- `core/youtubebot/live_chat.go`
- `core/repository/firestore_controller.go`

## 9. コメントの改善

### 現状の問題点
- 一部のコードにコメントが不足している
- 日本語と英語のコメントが混在している

### 改善案
- 複雑なロジックに対する説明コメントの追加
- コメント言語の統一（できれば英語に）
- GoDocスタイルのコメントの追加

```go
// CheckLongTimeSitting checks for users who have been sitting for too long
// and moves them to different seats if necessary.
// It processes both member and general seats based on the isMemberRoom parameter.
//
// Parameters:
//   - ctx: The context for the operation
//   - isMemberRoom: Whether to check member seats (true) or general seats (false)
//
// Returns:
//   - error: Any error that occurred during the process
func (s *System) CheckLongTimeSitting(ctx context.Context, isMemberRoom bool) error {
    // 実装...
}
```

### 対象ファイル
- すべてのソースファイル

## 10. 並行処理の改善

### 現状の問題点
- `GoroutineCheckLongTimeSitting` などの並行処理パターンが最適でない可能性
- エラーハンドリングが不十分

### 改善案
- コンテキストを使ったキャンセル処理の改善
- エラーハンドリングの強化

```go
// GoroutineCheckLongTimeSitting 長時間座席占有検出ループ
func (s *System) GoroutineCheckLongTimeSitting(ctx context.Context) {
    minimumInterval := time.Duration(s.Configs.Constants.MinimumCheckLongTimeSittingIntervalMinutes) * time.Minute
    slog.InfoContext(ctx, "Starting long time sitting check routine", "interval", minimumInterval)

    ticker := time.NewTicker(minimumInterval)
    defer ticker.Stop()

    // 初回実行
    s.runLongTimeSittingCheck(ctx)

    for {
        select {
        case <-ctx.Done():
            slog.InfoContext(ctx, "Long time sitting check routine stopped due to context cancellation")
            return
        case <-ticker.C:
            s.runLongTimeSittingCheck(ctx)
        }
    }
}

func (s *System) runLongTimeSittingCheck(ctx context.Context) {
    slog.InfoContext(ctx, "Running long time sitting check")
    start := utils.JstNow()

    if err := s.CheckLongTimeSitting(ctx, true); err != nil {
        s.MessageToOwnerWithError(ctx, "Error in CheckLongTimeSitting for member seats", err)
    }

    if err := s.CheckLongTimeSitting(ctx, false); err != nil {
        s.MessageToOwnerWithError(ctx, "Error in CheckLongTimeSitting for general seats", err)
    }

    end := utils.JstNow()
    duration := end.Sub(start)
    slog.InfoContext(ctx, "Completed long time sitting check", "duration", duration)
}
```

### 対象ファイル
- `core/system.go` - 特に `GoroutineCheckLongTimeSitting` 関数

## 実装の優先順位

1. エラーハンドリングの改善（最も影響が大きい）
2. リトライロジックの抽象化（重複コードの削減）
3. 長い関数の分割（コードの可読性向上）
4. マジックナンバー・文字列の定数化（保守性向上）
5. ロギングの統一（デバッグ容易性向上）
6. コンテキスト伝播の改善
7. テスト容易性の向上
8. コメントの改善
9. 並行処理の改善
10. フィールド名の統一と整合性確保

これらの改善を実施することで、コードの可読性、保守性、テスト容易性が向上し、将来の機能追加や変更がしやすくなります。

## 11. フィールド名の統一と整合性確保

### 現状の問題点
- コード内で `userDoc.CreatedAt` を参照しているが、実際の `UserDoc` 構造体には `RegistrationDate` フィールドが定義されている
- フィールド名の不一致によりコンパイルエラーが発生している
- 同様の命名の不一致が他の構造体にも存在する可能性がある

### 改善案
- フィールド名の統一
  - `UserDoc` 構造体の `RegistrationDate` を `CreatedAt` に変更する、または
  - コード内の `userDoc.CreatedAt` の参照を `userDoc.RegistrationDate` に修正する

```go
// 例: exitRoom関数内の修正
if seatDoc.WorkStartedAt.After(userDoc.RegistrationDate) {
    // 処理...
}
```

- 構造体フィールド名の命名規則の統一と文書化
- 全コードベースでの命名の一貫性チェック

### 対象ファイル
- `core/repository/models.go` - `UserDoc` 構造体の定義
- `core/system.go` - `exitRoom` 関数など、`UserDoc` を使用している箇所
- 新規作成された `core/system/user.go` - ユーザー関連の処理を含むファイル
