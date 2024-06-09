'use client'

import { useRouter } from 'next/navigation'
import { useCallback } from 'react'
import Link from 'next/link'

export default function Home() {
    const router = useRouter()

    const onClickLogin = useCallback(() => {
        router.push('/sign-in')
    }, [])

    return (
        <main>
            <div className="flex flex-col justify-center">
                <div>
                    <h1 className="text-xl font-semibold">Home</h1>
                </div>

                <div>
                    <button
                        type="button"
                        onClick={onClickLogin}
                        className="rounded-full bg-blue-500 px-4 py-2 font-bold text-white hover:bg-blue-700"
                    >
                        ログイン
                    </button>
                </div>

                <div>
                    <Link href="/userinfo">ユーザー情報を見る</Link>
                </div>
            </div>
        </main>
    )
}
