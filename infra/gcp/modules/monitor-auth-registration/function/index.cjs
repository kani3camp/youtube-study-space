'use strict'

const { beforeUserCreated, HttpsError } = require('firebase-functions/v2/identity')
const { rejectPasswordSignup } = require('./guard.cjs')

// Firebase公式SDKが署名付きブロッキングイベントを検証するHTTPエントリーポイント。
// 配備とbeforeCreateの登録はTerraformが所有する。Firebase CLIから配備しない。
exports.blockPasswordSignup = beforeUserCreated((event) => rejectPasswordSignup(event, HttpsError))
