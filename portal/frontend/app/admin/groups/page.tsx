import { auth } from "@/auth"
import { GroupsManager } from "@/components/admin/groups-manager"

export default async function AdminGroupsPage() {
  const session = await auth()
  return <GroupsManager accessToken={session?.accessToken} />
}
