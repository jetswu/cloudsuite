import { auth } from "@/auth"

const SUPER_ADMIN_GROUP = "cloudsuite-superadmin"

export default auth((req) => {
  const { nextUrl } = req
  const isLoggedIn = Boolean(req.auth?.user)

  if (!isLoggedIn) {
    return Response.redirect(new URL("/login", nextUrl))
  }

  const groups = req.auth?.user?.groups ?? []
  if (nextUrl.pathname.startsWith("/admin") && !groups.includes(SUPER_ADMIN_GROUP)) {
    return Response.redirect(new URL("/dashboard", nextUrl))
  }
})

export const config = {
  matcher: ["/dashboard/:path*", "/admin/:path*"],
}
