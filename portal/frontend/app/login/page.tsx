import { signIn } from "@/auth"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export default function LoginPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">CloudSuite</CardTitle>
          <CardDescription>
            Masuk menggunakan akun CloudSuite Anda
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            action={async () => {
              "use server"
              await signIn("authentik", { redirectTo: "/dashboard" })
            }}
          >
            <Button className="w-full" type="submit">
              Masuk dengan Authentik
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
