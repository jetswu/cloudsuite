import { auth } from "@/auth"
import { DomainSetup } from "@/components/admin/domain-setup"

export default async function DomainSetupPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const session = await auth()
  const { id } = await params
  return <DomainSetup accessToken={session?.accessToken} domainId={id} />
}
