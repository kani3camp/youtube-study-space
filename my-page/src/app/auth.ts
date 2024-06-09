import GoogleProvider from 'next-auth/providers/google'
import NextAuth from 'next-auth'

export const { handlers, signIn, signOut, auth } = NextAuth({
    providers: [
        GoogleProvider({
            clientId: process.env.GOOGLE_ID,
            clientSecret: process.env.GOOGLE_SECRET,
            authorization: {
                params: {
                    prompt: 'consent',
                    access_type: 'offline',
                    response_type: 'code',
                    scope: 'https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/youtube.readonly',
                },
            },
        }),
    ],
    pages: {
        signIn: '/sign-in',
        // signOut: '/sign-out',
    },
    callbacks: {
        jwt: async ({ token, user, account, profile, isNewUser }) => {
            if (user) {
                token.user = user
                const u = user as any
                token.role = u.role
            }
            if (account) {
                token.accessToken = account.access_token
                token.refreshToken = account.refresh_token
            }
            return token
        },
        session: ({ session, token }) => {
            token.accessToken
            return {
                ...session,
                user: {
                    ...session.user,
                    role: token.role,
                    accessToken: token.accessToken,
                    refreshToken: token.refreshToken,
                },
            }
        },
    },
})
