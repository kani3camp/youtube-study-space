'use client'

import { useCallback } from 'react'

export default function Home() {
    const onClickLogin = useCallback(() => {
        alert('clicked')
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
            </div>
        </main>
    )
}
