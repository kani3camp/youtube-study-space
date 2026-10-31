'use strict'

const assert = require('node:assert/strict')
const { test } = require('node:test')
const { HttpsError } = require('firebase-functions/v2/identity')
const { rejectPasswordSignup } = require('./guard.cjs')

test('クライアントからのメール・パスワード登録を拒否する', () => {
  assert.throws(
    () => rejectPasswordSignup({ eventType: 'providers/cloud.auth/eventTypes/user.beforeCreate:password' }, HttpsError),
    (error) => error instanceof HttpsError && error.code === 'permission-denied',
  )
})

for (const provider of ['google.com', 'custom', 'oidc.example']) {
  test(`${provider}の登録を拒否しない`, () => {
    assert.doesNotThrow(() => rejectPasswordSignup({ eventType: `providers/cloud.auth/eventTypes/user.beforeCreate:${provider}` }, HttpsError))
  })
}

test('HTTPエントリーポイントはFirebaseのbeforeCreateハンドラーとして公開する', () => {
  const { blockPasswordSignup } = require('./index.cjs')
  assert.equal(typeof blockPasswordSignup, 'function')
  assert.ok(blockPasswordSignup.__endpoint.blockingTrigger)
  assert.equal(blockPasswordSignup.__endpoint.blockingTrigger.eventType, 'providers/cloud.auth/eventTypes/user.beforeCreate')
})

test('署名のない外部リクエストでは関数を成功させない', async () => {
  process.env.GCLOUD_PROJECT = 'demo-monitor-auth-registration'
  const { blockPasswordSignup } = require('./index.cjs')
  const encode = (value) => Buffer.from(JSON.stringify(value)).toString('base64url')
  const jwt = `${encode({ alg: 'none' })}.${encode({ event_type: 'beforeCreate', sign_in_method: 'google.com', aud: 'https://fake.run.app' })}.`
  let status
  const res = {
    setHeader() {},
    status(code) { status = code; return this },
    send() {},
  }
  await blockPasswordSignup({ method: 'POST', header: () => 'application/json', body: { data: { jwt } } }, res)
  assert.ok(status >= 400)
})
