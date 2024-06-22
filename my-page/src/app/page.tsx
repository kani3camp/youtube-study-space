import Link from 'next/link'
import { SignIn } from './components/sign-in'
import { auth } from './auth'
import { SignOut } from './components/sign-out'

export default async function Home() {
    const session = await auth()

    return (
        <main>
            <div className="flex flex-col justify-center">
                <div>
                    <h1 className="text-xl font-semibold">Home</h1>
                </div>

                {!session && <SignIn></SignIn>}

                {session && (
                    <div>
                        <Link
                            href="/userinfo"
                            className="mx-3 my-5 rounded-full bg-blue-500 px-4 py-2 font-bold text-white hover:bg-blue-700"
                        >
                            ユーザー情報を見る
                        </Link>
                    </div>
                )}

                {session && <SignOut></SignOut>}
            </div>
        </main>
    )
}
