import { BarChart3, Cloud, HardDrive, Mail } from "lucide-react";
import { auth, signOut } from "@/auth";
import { ServiceCard } from "@/components/service-card";
import { UserMenu } from "@/components/user-menu";
import { MeStatus } from "@/components/me-status";

const SUPER_ADMIN_GROUP = "cloudsuite-superadmin";

const services = [
  {
    icon: Mail,
    name: "Mail",
    description: "Webmail Stalwart untuk email organisasi Anda.",
    href: "https://webmail.idchsuite.my.id",
  },
  {
    icon: HardDrive,
    name: "Drive",
    description: "Penyimpanan file dan kolaborasi lewat Nextcloud.",
    href: "https://drive.idchsuite.my.id",
  },
  {
    icon: BarChart3,
    name: "ERP",
    description: "Kelola operasional dan keuangan lewat Odoo.",
    href: "https://erp.idchsuite.my.id",
  },
];

export default async function DashboardPage() {
  const session = await auth();

  if (!session?.user) {
    return (
      <div className="flex min-h-screen items-center justify-center px-4">
        <div className="text-center">
          <p className="text-muted-foreground">Tidak terautentikasi.</p>
          <a
            className="mt-2 inline-block text-sm font-medium text-primary underline-offset-4 hover:underline"
            href="/login"
          >
            Masuk
          </a>
        </div>
      </div>
    );
  }

  const today = new Intl.DateTimeFormat("id-ID", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  }).format(new Date());

  const signOutAction = async () => {
    "use server";
    await signOut({ redirectTo: "/login" });
  };

  return (
    <div className="flex min-h-screen flex-col">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b border-border bg-background/80 backdrop-blur">
        <div className="mx-auto flex h-16 w-full max-w-5xl items-center justify-between px-4">
          <a href="/dashboard" className="flex items-center gap-2">
            <span className="flex size-8 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <Cloud className="size-5" aria-hidden />
            </span>
            <span className="text-lg font-semibold tracking-tight">CloudSuite</span>
          </a>
          <UserMenu
            name={session.user?.name}
            email={session.user?.email}
            isSuperAdmin={session.user?.groups?.includes(SUPER_ADMIN_GROUP) ?? false}
            signOutAction={signOutAction}
          />
        </div>
      </header>

      {/* Main */}
      <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-10">
        <div className="space-y-8">
          <div className="space-y-2">
            <h1 className="text-2xl font-semibold tracking-tight">
              Selamat datang{session.user?.name ? `, ${session.user.name}` : ""}.
            </h1>
            <p className="text-sm text-muted-foreground">{today}</p>
          </div>

          <MeStatus accessToken={session.accessToken} />

          <section className="space-y-4">
            <div className="flex items-baseline justify-between">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                Layanan
              </h2>
            </div>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {services.map((s) => (
                <ServiceCard key={s.name} {...s} />
              ))}
            </div>
          </section>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-border">
        <div className="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-6 text-xs text-muted-foreground">
          <span>CloudSuite v0.1.0</span>
          <span>© 2026 PT IDCloudHost</span>
        </div>
      </footer>
    </div>
  );
}
