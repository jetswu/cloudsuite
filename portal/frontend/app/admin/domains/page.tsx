import { auth } from "@/auth"
import { DomainsManager } from "@/components/admin/domains-manager"

export default async function AdminDomainsPage() {
  const session = await auth()
  return <DomainsManager accessToken={session?.accessToken} />
}
