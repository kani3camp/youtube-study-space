import {
	assertFails,
	assertSucceeds,
	initializeTestEnvironment,
} from '@firebase/rules-unit-testing'
import { collection, doc, getDoc, getDocs, setDoc } from 'firebase/firestore'
import { after, before, beforeEach, describe, test } from 'node:test'
import { readFile } from 'node:fs/promises'

const projectId = 'demo-youtube-study-space-rules'
const firestoreRules = await readFile(
	new URL('./firestore.rules', import.meta.url),
	'utf8',
)

let testEnvironment

before(async () => {
	testEnvironment = await initializeTestEnvironment({
		projectId,
		firestore: { rules: firestoreRules },
	})
})

beforeEach(async () => {
	await testEnvironment.clearFirestore()
	await testEnvironment.withSecurityRulesDisabled(async (context) => {
		const firestore = context.firestore()
		for (const path of [
			'config/constants',
			'config/credentials',
			'config/unknown',
			'public-config/monitor',
			'public-config/unknown',
			'seats/1',
			'member-seats/1',
			'menu/default',
			'work-name-trend/current',
			'users/private-user',
			'live-chat-history/private-message',
			'work-segments/private-segment',
			'unknown/private-document',
		]) {
			await setDoc(doc(firestore, path), { seeded: true })
		}
	})
})

after(async () => {
	await testEnvironment.cleanup()
})

const anonymousFirestore = () =>
	testEnvironment.unauthenticatedContext().firestore()

describe('public documents', () => {
	for (const path of [
		'public-config/monitor',
		'seats/1',
		'member-seats/1',
		'menu/default',
		'work-name-trend/current',
	]) {
		test(`${path} allows anonymous reads and denies client writes`, async () => {
			const reference = doc(anonymousFirestore(), path)

			await assertSucceeds(getDoc(reference))
			await assertFails(setDoc(reference, { clientWrite: true }))
		})
	}
})

describe('private documents', () => {
	for (const path of [
		'config/constants',
		'config/credentials',
		'config/unknown',
		'public-config/unknown',
		'users/private-user',
		'live-chat-history/private-message',
		'work-segments/private-segment',
		'unknown/private-document',
	]) {
		test(`${path} denies anonymous reads and writes`, async () => {
			const reference = doc(anonymousFirestore(), path)

			await assertFails(getDoc(reference))
			await assertFails(setDoc(reference, { clientWrite: true }))
		})
	}

	test('config collection cannot be listed by an anonymous client', async () => {
		await assertFails(getDocs(collection(anonymousFirestore(), 'config')))
	})

	test('arbitrary authentication does not grant access to any config document', async () => {
		const authenticatedFirestore = testEnvironment
			.authenticatedContext('arbitrary-user')
			.firestore()
		for (const path of [
			'config/constants',
			'config/credentials',
			'config/unknown',
		]) {
			const reference = doc(authenticatedFirestore, path)

			await assertFails(getDoc(reference))
			await assertFails(setDoc(reference, { clientWrite: true }))
		}
		await assertFails(getDocs(collection(authenticatedFirestore, 'config')))
	})
})
