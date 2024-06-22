import { signOut } from '@/app/auth'
import Link from 'next/link'

export function SignOut() {
    return (
        <>
            <form
                action={async () => {
                    'use server'
                    await signOut()
                }}
            >
                <button
                    type="submit"
                    className="mx-3 my-5 rounded-full bg-blue-500 px-4 py-2 font-bold text-white hover:bg-blue-700"
                >
                    サインアウト
                </button>
            </form>
        </>
    )
}
