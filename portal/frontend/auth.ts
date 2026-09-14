import NextAuth from "next-auth"
import Authentik from "next-auth/providers/authentik"

export const { handlers, auth, signIn, signOut } = NextAuth({
  providers: [
    Authentik({
      clientId: process.env.AUTHENTIK_CLIENT_ID!,
      clientSecret: process.env.AUTHENTIK_CLIENT_SECRET!,
      issuer: process.env.AUTHENTIK_ISSUER,
    }),
  ],
  pages: {
    signIn: "/login",
  },
  session: {
    strategy: "jwt",
  },
  callbacks: {
    async jwt({ token, account, profile }) {
      if (account?.access_token) {
        token.accessToken = account.access_token
      }
      if (profile) {
        token.sub = profile.sub as string
        token.email = (profile.email as string) ?? token.email
        token.name = (profile.name as string) ?? token.name
        token.preferred_username =
          (profile.preferred_username as string) ?? token.preferred_username
        // Authentik "profile" scope mapping emits group names here.
        token.groups = (profile as { groups?: string[] }).groups ?? []
      }
      return token
    },
    async session({ session, token }) {
      session.accessToken = token.accessToken as string | undefined
      if (session.user) {
        session.user.sub = token.sub as string
        session.user.preferred_username =
          token.preferred_username as string | undefined
        session.user.groups = token.groups as string[] | undefined
      }
      return session
    },
  },
})
