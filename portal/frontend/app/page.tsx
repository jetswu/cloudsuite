import { auth, signIn } from "@/auth"
import { Button } from "@/components/ui/button"

export default async function HomePage() {
  const session = await auth()

  if (session?.user) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4">
        <p>Sudah masuk sebagai {session.user?.email}</p>
        <a className="underline" href="/dashboard">
          Ke Dashboard
        </a>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4">
      <h1 className="text-3xl font-semibold">CloudSuite</h1>
      <p className="text-muted-foreground">Portal layanan CloudSuite</p>
      <form
        action={async () => {
          "use server"
          await signIn("authentik", { redirectTo: "/dashboard" })
        }}
      >
        <Button type="submit">Masuk</Button>
      </form>
    </div>
  )
}
