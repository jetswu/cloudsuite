import { auth } from "@/auth"
import { AuditManager } from "@/components/admin/audit-manager"

export default async function AdminAuditPage() {
  const session = await auth()
  return <AuditManager accessToken={session?.accessToken} />
}
