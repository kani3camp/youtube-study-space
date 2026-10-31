'use strict'

const assert = require('node:assert/strict')
const { before, after, test } = require('node:test')
const { createServer } = require('node:http')
const { once } = require('node:events')

const project = 'demo-monitor-auth-registration'
const emulator = process.env.FIREBASE_AUTH_EMULATOR_HOST
assert.match(emulator ?? '', /^127\.0\.0\.1:\d+$/)
assert.equal(process.env.GCLOUD_PROJECT, project)
// Auth Emulatorはイベントの署名を行わない。このテストプロセスにだけ適用する。
process.env.FIREBASE_DEBUG_MODE = 'true'
process.env.FIREBASE_DEBUG_FEATURES = JSON.stringify({ skipTokenVerification: true })
const { blockPasswordSignup } = require('./index.cjs')
const { initializeApp, deleteApp } = require('firebase-admin/app')
const { getAuth } = require('firebase-admin/auth')
const app = initializeApp({ projectId: project }, 'integration-admin')
const auth = getAuth(app)
let server
let calls = 0

async function request(path, body, method = 'POST', admin = false) {
  const response = await fetch(`http://${emulator}/identitytoolkit.googleapis.com/${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', ...(admin ? { Authorization: 'Bearer owner' } : {}) },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(10000),
  })
  return { status: response.status, data: await response.json() }
}

before(async () => {
  await auth.createUser({ uid: 'preexisting-monitor', email: 'existing@example.test', password: 'synthetic-test-password' })
  server = createServer(async (req, res) => {
    calls++
    let raw = ''
    for await (const chunk of req) raw += chunk
    req.body = JSON.parse(raw)
    req.header = (name) => req.headers[name.toLowerCase()]
    res.status = (code) => { res.statusCode = code; return res }
    res.send = (value) => res.end(typeof value === 'string' ? value : JSON.stringify(value))
    await blockPasswordSignup(req, res)
  }).listen(0, '127.0.0.1')
  await once(server, 'listening')
  const config = await request(`v2/projects/${project}/config?updateMask=blockingFunctions`, {
    blockingFunctions: { triggers: { beforeCreate: { functionUri: `http://127.0.0.1:${server.address().port}` } } },
  }, 'PATCH', true)
  assert.equal(config.status, 200)
})

after(async () => {
  if (server) await new Promise((resolve) => server.close(resolve))
  await deleteApp(app)
})

test('公開APIによるメール・パスワード登録は保存前に拒否する', async () => {
  const result = await request('v1/accounts:signUp?key=demo-key', {
    email: 'public-signup@example.test', password: 'synthetic-test-password', returnSecureToken: true,
  })
  assert.equal(result.status, 400)
  assert.match(result.data.error.message, /BLOCKING_FUNCTION_ERROR_RESPONSE/)
  await assert.rejects(auth.getUserByEmail('public-signup@example.test'), { code: 'auth/user-not-found' })
  assert.equal(calls, 1)
})

test('コンソール相当の管理者作成と既存ユーザーのパスワードログインを維持する', async () => {
  const before = calls
  await auth.createUser({ uid: 'admin-created-monitor', email: 'admin@example.test', password: 'synthetic-test-password' })
  for (const email of ['existing@example.test', 'admin@example.test']) {
    const result = await request('v1/accounts:signInWithPassword?key=demo-key', {
      email, password: 'synthetic-test-password', returnSecureToken: true,
    })
    assert.equal(result.status, 200)
    assert.ok(result.data.idToken)
  }
  assert.equal(calls, before)
})

test('マイページと同じカスタム認証で初回ユーザーを作成できる', async () => {
  const before = calls
  const uid = 'UCsynthetic0000000000001'
  const result = await request('v1/accounts:signInWithCustomToken?key=demo-key', {
    token: JSON.stringify({ uid }), returnSecureToken: true,
  })
  assert.equal(result.status, 200)
  assert.equal(result.data.isNewUser, true)
  assert.equal((await auth.getUser(uid)).uid, uid)
  assert.equal(calls, before)
})
