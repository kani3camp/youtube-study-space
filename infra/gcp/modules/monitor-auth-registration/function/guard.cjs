'use strict'

// beforeCreateだけに接続する。既存ユーザーのログインや、マイページのcustom認証を拒否しない。
function rejectPasswordSignup(event, HttpsError) {
  if (event.eventType?.endsWith(':password')) {
    throw new HttpsError('permission-denied', 'メール・パスワードでの新規登録は受け付けていません。')
  }
}

module.exports = { rejectPasswordSignup }
