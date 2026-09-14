import { redirect } from "next/navigation"
import { auth, signOut } from "@/auth"
import { AdminShell } from "@/components/admin/admin-shell"
import type { ReactNode } from "react"

const SUPER_ADMIN_GROUP = "cloudsuite-superadmin"

export default async function AdminLayout({
  children,
}: {
  children: ReactNode
}) {
  const session = await auth()

  if (!session?.user) {
    redirect("/login")
  }

  if (!session.user.groups?.includes(SUPER_ADMIN_GROUP)) {
    redirect("/dashboard")
  }

  const signOutAction = async () => {
    "use server"
    await signOut({ redirectTo: "/login" })
  }

  return (
    <AdminShell
      name={session.user.name}
      email={session.user.email}
      signOutAction={signOutAction}
    >
      {children}
    </AdminShell>
  )
}
