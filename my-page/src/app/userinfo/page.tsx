import { auth } from '@/app/auth'
import { google, youtube_v3 } from 'googleapis'
import Link from 'next/link'

export default async function UserInfoPage() {
    const session = await auth()
    const user = session?.user as any

    const oauth2Client = new google.auth.OAuth2({
        clientId: process.env.GOOGLE_CLIENT_ID,
        clientSecret: process.env.GOOGLE_CLIENT_SECRET,
        // redirectUri: 'http://localhost:3000/api/auth/callback/google',
    })

    const accessToken = user?.accessToken
    if (!accessToken) {
        return <div>accessToken is null</div>
    }

    // トークンを設定。refresh_tokenも渡せます。
    oauth2Client.setCredentials({ access_token: accessToken })

    const youtube: youtube_v3.Youtube = google.youtube({
        version: 'v3',
        auth: oauth2Client,
    })

    const response = await youtube.channels.list({ part: ['id'], mine: true })
    const channelId = response.data.items?.at(0)?.id

    return (
        <>
            <div>あなたのチャンネルID： {channelId}</div>

            <Link
                href="/"
                className="mx-3 my-5 rounded-full bg-blue-500 px-4 py-2 font-bold text-white hover:bg-blue-700"
            >
                Homeに戻る
            </Link>
        </>
    )
}
