import { auth } from "@/auth"
import { UsersManager } from "@/components/admin/users-manager"

export default async function AdminUsersPage() {
  const session = await auth()
  return <UsersManager accessToken={session?.accessToken} />
}
