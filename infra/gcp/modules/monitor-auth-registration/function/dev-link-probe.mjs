import { readFile } from 'node:fs/promises'
import { randomUUID, randomBytes } from 'node:crypto'
import { spawnSync } from 'node:child_process'
import { isDeepStrictEqual } from 'node:util'
import { createRequire } from 'node:module'

const project = 'test-youtube-study-space'
if (process.argv.slice(2).join(' ') !== '--target dev') throw new Error('DEV target is required')
let phase = 'configuration'
let accessToken
let apiKey
let sdkApp
let deleteSdkApp
const owned = []
const evidence = []
class ProbeError extends Error {
  constructor(status) { super('Probe failed'); this.status = status }
}
function requireThat(ok, status = 0) { if (!ok) throw new ProbeError(status) }
async function request(path, body, admin = false, fields) {
  const url = new URL(`https://identitytoolkit.googleapis.com/${path}`)
  if (!admin) url.searchParams.set('key', apiKey)
  if (fields) url.searchParams.set('fields', fields)
  const response = await fetch(url, {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'Content-Type': 'application/json', ...(admin ? { Authorization: `Bearer ${accessToken}`, 'X-Goog-User-Project': project } : {}) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    signal: AbortSignal.timeout(30000),
  })
  const data = await response.json()
  return { status: response.status, data }
}
const configFields = 'subtype,signIn.email.enabled,signIn.email.passwordRequired,blockingFunctions.triggers,emailPrivacyConfig.enableImprovedEmailPrivacy,client.permissions.disabledUserSignup'
async function config() {
  const result = await request(`admin/v2/projects/${project}/config`, undefined, true, configFields)
  requireThat(result.status === 200, result.status)
  requireThat(result.data.subtype === 'IDENTITY_PLATFORM')
  requireThat(result.data.signIn?.email?.enabled === true)
  requireThat(typeof result.data.blockingFunctions?.triggers?.beforeCreate?.functionUri === 'string')
  requireThat(result.data.blockingFunctions.triggers.beforeCreate.functionUri.includes('monitor-password-signup-guard'))
  return result.data
}
async function providerState(idToken) {
  const result = await request('v1/accounts:lookup', { idToken }, false, 'users(localId,providerUserInfo(providerId))')
  requireThat(result.status === 200, result.status)
  requireThat(result.data.users?.length === 1)
  return { uid: result.data.users[0].localId, hasPassword: result.data.users[0].providerUserInfo?.some((p) => p.providerId === 'password') === true }
}
let failed = false
try {
  const raw = await readFile(new URL('../../../../../youtube-monitor/.env', import.meta.url), 'utf8')
  const entries = Object.fromEntries(raw.split(/\r?\n/).filter((line) => line.startsWith('NEXT_PUBLIC_FIREBASE_') && line.includes('=')).map((line) => {
    const at = line.indexOf('=')
    return [line.slice(0, at), line.slice(at + 1).trim().replace(/^(['"])(.*)\1$/, '$2')]
  }))
  requireThat(entries.NEXT_PUBLIC_FIREBASE_PROJECT_ID === project)
  apiKey = entries.NEXT_PUBLIC_FIREBASE_API_KEY
  requireThat(typeof apiKey === 'string' && apiKey.length > 10)
  phase = 'verify-api-key-project'
  const identity = await request('v1/projects', undefined, false, 'projectId')
  requireThat(identity.status === 200, identity.status)
  const metadata = spawnSync('gcloud', ['projects', 'describe', project, '--format=json(projectId,projectNumber)', '--quiet'], { encoding: 'utf8', timeout: 30000 })
  requireThat(metadata.status === 0)
  const target = JSON.parse(metadata.stdout)
  requireThat(target.projectId === project && /^[0-9]+$/.test(String(target.projectNumber)))
  requireThat(identity.data.projectId === project || String(identity.data.projectId) === String(target.projectNumber))
  phase = 'admin-credentials'
  const credential = spawnSync('gcloud', ['auth', 'print-access-token', `--project=${project}`, '--quiet'], { encoding: 'utf8', timeout: 30000 })
  requireThat(credential.status === 0)
  accessToken = credential.stdout.trim()
  requireThat(accessToken.length > 0)
  phase = 'auth-config-read'
  const before = await config()
  const require = createRequire(import.meta.url)
  const { initializeApp, deleteApp } = require('firebase-admin/app')
  const { getAuth } = require('firebase-admin/auth')
  deleteSdkApp = deleteApp
  sdkApp = initializeApp({
    projectId: project,
    serviceAccountId: `monitor-auth-runtime@${project}.iam.gserviceaccount.com`,
    credential: { getAccessToken: async () => ({ access_token: accessToken, expires_in: 3600 }) },
  }, 'dev-link-probe')
  const signer = getAuth(sdkApp)
  let signingAvailable
  for (const endpoint of ['signUp', 'update']) {
    const suffix = randomUUID()
    const uid = `monitor-password-link-probe-${suffix}`
    const seedEmail = `monitor-seed-${suffix}@example.test`
    const email = `monitor-linked-${suffix}@example.test`
    const initialPassword = randomBytes(24).toString('hex')
    const password = randomBytes(24).toString('hex')
    phase = `${endpoint}:verify-uid-unused`
    const unused = await request(`v1/projects/${project}/accounts:lookup`, { localId: [uid] }, true, 'users(localId)')
    requireThat(unused.status === 200 && !unused.data.users?.length, unused.status)
    // 書込み応答がタイムアウトした場合も、この未使用の検証UIDをfinallyで確認する。
    owned.push(uid)
    let customToken
    if (signingAvailable !== false) {
      phase = `${endpoint}:short-lived-custom-token`
      try {
        customToken = await signer.createCustomToken(uid)
        signingAvailable = true
      } catch {
        // 権限を追加せず、既存権限で署名できない場合は別の検証用経路に限定する。
        signingAvailable = false
      }
    }
    let token
    let origin
    if (customToken) {
      phase = `${endpoint}:custom-auth-first-login`
      const signedIn = await request('v1/accounts:signInWithCustomToken', { token: customToken, returnSecureToken: true })
      requireThat(signedIn.status === 200 && signedIn.data.localId === uid && typeof signedIn.data.idToken === 'string', signedIn.status)
      token = signedIn.data.idToken
      origin = 'custom-auth'
    } else {
      phase = `${endpoint}:create-owned-user`
      const created = await request(`v1/projects/${project}/accounts`, { localId: uid, email: seedEmail, password: initialPassword }, true, 'localId')
      requireThat(created.status === 200 && created.data.localId === uid, created.status)
      phase = `${endpoint}:seed-login`
      const signedIn = await request('v1/accounts:signInWithPassword', { email: seedEmail, password: initialPassword, returnSecureToken: true })
      requireThat(signedIn.status === 200 && signedIn.data.localId === uid && typeof signedIn.data.idToken === 'string', signedIn.status)
      phase = `${endpoint}:unlink-password`
      const removed = await request('v1/accounts:update', { idToken: signedIn.data.idToken, deleteProvider: ['password'], returnSecureToken: true })
      requireThat(removed.status === 200, removed.status)
      token = removed.data.idToken ?? signedIn.data.idToken
      origin = 'admin-seeded-password-unlinked'
    }
    const withoutPassword = await providerState(token)
    requireThat(withoutPassword.uid === uid && withoutPassword.hasPassword === false)
    phase = `${endpoint}:link-password`
    const linked = await request(`v1/accounts:${endpoint}`, { idToken: token, email, password, returnSecureToken: true })
    const record = { endpoint, origin, linkHttpStatus: linked.status }
    if (linked.status === 200) {
      const newToken = linked.data.idToken ?? token
      const linkedState = await providerState(newToken)
      requireThat(linkedState.uid === uid && linkedState.hasPassword === true)
      phase = `${endpoint}:linked-password-login`
      const login = await request('v1/accounts:signInWithPassword', { email, password, returnSecureToken: true })
      requireThat(login.status === 200 && login.data.localId === uid, login.status)
      record.passwordProviderAdded = true
      record.passwordLoginSucceeded = true
    } else {
      const code = String(linked.data.error?.message ?? '').split(/[ :]/, 1)[0]
      record.errorCode = /^[A-Z_]{1,80}$/.test(code) ? code : 'OTHER'
      record.passwordProviderAdded = (await providerState(token)).hasPassword
    }
    evidence.push(record)
  }
  phase = 'auth-config-unchanged'
  requireThat(isDeepStrictEqual(before, await config()))
  console.log(JSON.stringify({ project, evidence, authConfigUnchanged: true, emailEnumerationProtection: before.emailPrivacyConfig?.enableImprovedEmailPrivacy === true }))
} catch (error) {
  failed = true
  console.error(JSON.stringify({ project, phase, failed: true, httpStatus: error instanceof ProbeError ? error.status : null }))
} finally {
  let cleanupFailed = false
  for (const uid of owned) {
    try {
      const exists = await request(`v1/projects/${project}/accounts:lookup`, { localId: [uid] }, true, 'users(localId)')
      requireThat(exists.status === 200, exists.status)
      if (!exists.data.users?.length) continue
      const deleted = await request(`v1/projects/${project}/accounts:delete`, { localId: uid }, true)
      requireThat(deleted.status === 200, deleted.status)
      const lookup = await request(`v1/projects/${project}/accounts:lookup`, { localId: [uid] }, true, 'users(localId)')
      requireThat(lookup.status === 200 && !lookup.data.users?.length, lookup.status)
    } catch {
      cleanupFailed = true
      // 無作為に生成した検証UIDだけを示し、管理者による後片付けを可能にする。
      console.error(JSON.stringify({ cleanupFailed: true, syntheticProbeUid: uid }))
    }
  }
  if (sdkApp && deleteSdkApp) await deleteSdkApp(sdkApp)
  console.log(JSON.stringify({ ownedProbeUids: owned.length, cleanupVerified: !cleanupFailed, residualOwnedUsers: cleanupFailed ? 'unknown' : 0 }))
  accessToken = undefined
  apiKey = undefined
  if (failed || cleanupFailed) process.exitCode = 1
}
