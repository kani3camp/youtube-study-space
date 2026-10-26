import { readFile } from 'node:fs/promises'
import { after, before, beforeEach, describe, test } from 'node:test'
import {
  assertFails,
  assertSucceeds,
  initializeTestEnvironment,
} from '@firebase/rules-unit-testing'
import {
  collection,
  doc,
  getDoc,
  getDocs,
  setDoc,
} from 'firebase/firestore'

const projectId = 'demo-youtube-study-space-ci'
const allowedUid = 'monitor-operator'
const rules = await readFile(new URL('./firestore.rules', import.meta.url), 'utf8')

let testEnv

before(async () => {
  testEnv = await initializeTestEnvironment({
    projectId,
    firestore: { rules },
  })
})

beforeEach(async () => {
  await testEnv.clearFirestore()
  await testEnv.withSecurityRulesDisabled(async (context) => {
    const db = context.firestore()
    await Promise.all([
      setDoc(doc(db, '_monitor-readers', allowedUid), { enabled: true }),
      setDoc(doc(db, '_monitor-readers', 'disabled-monitor'), {
        enabled: false,
      }),
      setDoc(doc(db, 'seats', '1'), { 'seat-id': 1 }),
      setDoc(doc(db, 'member-seats', '1'), { 'seat-id': 1 }),
      setDoc(doc(db, 'menu', 'coffee'), { code: 'coffee' }),
      setDoc(doc(db, 'work-name-trend', 'current'), { ranking: [] }),
      setDoc(doc(db, 'config', 'constants'), { 'max-seats': 1 }),
      setDoc(doc(db, 'config', 'private'), { secret: true }),
      setDoc(doc(db, 'unrelated', 'private'), { secret: true }),
      setDoc(doc(db, 'mypage-runtime-control', 'current'), {
        state: 'ACTIVE',
      }),
      setDoc(doc(db, 'mypage-maintenance-events', 'event'), {
        state: 'ACTIVE',
      }),
    ])
  })
})

after(async () => {
  await testEnv.cleanup()
})

const readableMonitorPaths = [
  ['seats', '1'],
  ['member-seats', '1'],
  ['menu', 'coffee'],
  ['work-name-trend', 'current'],
  ['config', 'constants'],
]

const readableMonitorCollections = [
  'seats',
  'member-seats',
  'menu',
  'work-name-trend',
]

describe('Firestore monitor read boundary', () => {
  test('allows only an allowlisted UID carrying the monitor claim', async () => {
    const db = testEnv
      .authenticatedContext(allowedUid, { monitor_reader: true })
      .firestore()

    for (const [collectionName, documentName] of readableMonitorPaths) {
      await assertSucceeds(getDoc(doc(db, collectionName, documentName)))
    }
    for (const collectionName of readableMonitorCollections) {
      await assertSucceeds(getDocs(collection(db, collectionName)))
    }
  })

  test('rejects every anonymous monitor read', async () => {
    const db = testEnv.unauthenticatedContext().firestore()

    for (const [collectionName, documentName] of readableMonitorPaths) {
      await assertFails(getDoc(doc(db, collectionName, documentName)))
    }
    for (const collectionName of readableMonitorCollections) {
      await assertFails(getDocs(collection(db, collectionName)))
    }
  })

  test('rejects an unlisted UID even when it carries the monitor claim', async () => {
    const db = testEnv
      .authenticatedContext('other-user', { monitor_reader: true })
      .firestore()

    await assertFails(getDoc(doc(db, 'seats', '1')))
    await assertFails(getDocs(collection(db, 'seats')))
  })

  test('rejects an allowlisted UID without the monitor claim', async () => {
    const db = testEnv.authenticatedContext(allowedUid).firestore()

    await assertFails(getDoc(doc(db, 'seats', '1')))
    await assertFails(getDocs(collection(db, 'seats')))
  })

  test('rejects a disabled allowlist entry', async () => {
    const db = testEnv
      .authenticatedContext('disabled-monitor', { monitor_reader: true })
      .firestore()

    await assertFails(getDoc(doc(db, 'seats', '1')))
  })

  test('never exposes the allowlist or unrelated documents', async () => {
    const db = testEnv
      .authenticatedContext(allowedUid, { monitor_reader: true })
      .firestore()

    await assertFails(getDoc(doc(db, '_monitor-readers', allowedUid)))
    await assertFails(getDocs(collection(db, '_monitor-readers')))
    await assertFails(getDoc(doc(db, 'config', 'private')))
    await assertFails(getDocs(collection(db, 'config')))
    await assertFails(getDoc(doc(db, 'unrelated', 'private')))
  })

  test('rejects browser writes even for an authorized monitor', async () => {
    const db = testEnv
      .authenticatedContext(allowedUid, { monitor_reader: true })
      .firestore()

    await assertFails(setDoc(doc(db, 'seats', '2'), { 'seat-id': 2 }))
    await assertFails(
      setDoc(doc(db, '_monitor-readers', 'other-user'), { enabled: true }),
    )
  })

  test('never exposes or mutates maintenance control and audit records', async () => {
    for (const db of [
      testEnv.unauthenticatedContext().firestore(),
      testEnv
        .authenticatedContext(allowedUid, { monitor_reader: true })
        .firestore(),
    ]) {
      await assertFails(getDoc(doc(db, 'mypage-runtime-control', 'current')))
      await assertFails(getDocs(collection(db, 'mypage-runtime-control')))
      await assertFails(getDoc(doc(db, 'mypage-maintenance-events', 'event')))
      await assertFails(getDocs(collection(db, 'mypage-maintenance-events')))
      await assertFails(
        setDoc(doc(db, 'mypage-runtime-control', 'current'), {
          state: 'NORMAL',
        }),
      )
      await assertFails(
        setDoc(doc(db, 'mypage-maintenance-events', 'forged'), {
          state: 'NORMAL',
        }),
      )
    }
  })
})
