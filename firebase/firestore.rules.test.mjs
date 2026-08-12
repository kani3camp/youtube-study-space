import {
	assertFails,
	assertSucceeds,
	initializeTestEnvironment,
} from '@firebase/rules-unit-testing'
import { doc, getDoc, setDoc } from 'firebase/firestore'
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
			'public-config/monitor',
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
		'config/constants',
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
		'config/credentials',
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

	test('arbitrary authentication does not grant access to private config', async () => {
		const authenticatedFirestore = testEnvironment
			.authenticatedContext('arbitrary-user')
			.firestore()
		const reference = doc(authenticatedFirestore, 'config/credentials')

		await assertFails(getDoc(reference))
		await assertFails(setDoc(reference, { clientWrite: true }))
	})
})
