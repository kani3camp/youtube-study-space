import { signIn } from '@/app/auth'
import Link from 'next/link'

export function SignIn() {
    return (
        <>
            <form
                action={async () => {
                    'use server'
                    await signIn('google')
                }}
            >
                <button
                    type="submit"
                    className="rounded-full bg-blue-500 px-4 py-2 font-bold text-white hover:bg-blue-700"
                >
                    Signin with Google
                </button>
            </form>
        </>
    )
}
