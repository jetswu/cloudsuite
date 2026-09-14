import { Cloud } from "lucide-react";
import { signIn } from "@/auth";
import { Button } from "@/components/ui/button";

const ERROR_MESSAGES: Record<string, string> = {
  OAuthSignin: "Gagal memulai proses masuk. Coba lagi.",
  OAuthCallback: "Gagal memverifikasi identitas Anda. Coba lagi.",
  Configuration: "Konfigurasi autentikasi bermasalah. Hubungi administrator.",
  AccessDenied: "Akses ditolak. Anda tidak diizinkan masuk.",
  Default: "Terjadi kesalahan saat masuk. Coba lagi.",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const { error } = await searchParams;

  return (
    <div className="flex min-h-screen">
      {/* Kiri — branding */}
      <div className="hidden flex-1 flex-col justify-between border-r border-border bg-card p-10 lg:flex">
        <div className="flex items-center gap-2">
          <span className="flex size-8 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Cloud className="size-5" aria-hidden />
          </span>
          <span className="text-lg font-semibold tracking-tight">CloudSuite</span>
        </div>

        <div className="space-y-6">
          <h1 className="max-w-md text-3xl font-semibold tracking-tight leading-tight">
            Satu login untuk semua layanan Anda.
          </h1>
          <ul className="space-y-3 text-sm text-muted-foreground">
            <li>Akses Mail, Drive, dan ERP dari satu akun.</li>
            <li>Satu identitas, satu password, semua layanan.</li>
            <li>Dikelola dan diawasi lewat satu portal.</li>
          </ul>
        </div>

        <p className="text-xs text-muted-foreground">
          © 2026 PT IDCloudHost
        </p>
      </div>

      {/* Kanan — form */}
      <div className="flex flex-1 items-center justify-center px-4 py-12">
        <div className="w-full max-w-sm space-y-8">
          <div className="flex flex-col items-center gap-2 lg:hidden">
            <span className="flex size-10 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <Cloud className="size-6" aria-hidden />
            </span>
            <span className="text-xl font-semibold tracking-tight">CloudSuite</span>
          </div>

          <div className="space-y-1.5 text-center lg:text-left">
            <h2 className="text-xl font-semibold tracking-tight">Masuk ke CloudSuite</h2>
            <p className="text-sm text-muted-foreground">
              Gunakan akun organisasi Anda untuk melanjutkan.
            </p>
          </div>

          {error ? (
            <div
              role="alert"
              className="rounded-md border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive"
            >
              {ERROR_MESSAGES[error] ?? ERROR_MESSAGES.Default}
            </div>
          ) : null}

          <form
            action={async () => {
              "use server";
              await signIn("authentik", { redirectTo: "/dashboard" });
            }}
          >
            <Button type="submit" size="lg" className="w-full">
              Sign in with CloudSuite
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}
