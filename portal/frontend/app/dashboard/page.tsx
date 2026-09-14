import { auth, signOut } from "@/auth"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

const services = [
  { name: "Mail", description: "Webmail Stalwart", href: "https://mail.idchsuite.my.id" },
  { name: "Drive", description: "Nextcloud", href: "https://drive.idchsuite.my.id" },
  { name: "ERP", description: "Odoo", href: "https://erp.idchsuite.my.id" },
]

async function getMe(accessToken?: string) {
  if (!accessToken) return null
  try {
    const res = await fetch(`${process.env.BACKEND_URL}/api/me`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      cache: "no-store",
    })
    if (!res.ok) return null
    return res.json()
  } catch {
    return null
  }
}

export default async function DashboardPage() {
  const session = await auth()
  if (!session?.user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>
          Tidak terautentikasi. <a className="underline" href="/login">Masuk</a>
        </p>
      </div>
    )
  }

  const me = await getMe(session.accessToken)

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-4">
          <h1 className="text-lg font-semibold">CloudSuite</h1>
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">
              {session.user?.email}
            </span>
            <form
              action={async () => {
                "use server"
                await signOut({ redirectTo: "/login" })
              }}
            >
              <Button variant="outline" size="sm" type="submit">
                Sign Out
              </Button>
            </form>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl space-y-6 px-4 py-8">
        {me ? (
          <p className="text-sm text-muted-foreground">
            Backend /api/me: sub={String(me.sub)} username=
            {String(me.preferred_username)}
          </p>
        ) : null}

        <div className="grid gap-4 sm:grid-cols-3">
          {services.map((s) => (
            <Card key={s.name}>
              <CardHeader>
                <CardTitle>{s.name}</CardTitle>
                <CardDescription>{s.description}</CardDescription>
              </CardHeader>
              <CardContent>
                <a className="text-sm underline" href={s.href} target="_blank" rel="noreferrer">
                  Buka {s.name}
                </a>
              </CardContent>
            </Card>
          ))}
        </div>
      </main>
    </div>
  )
}
