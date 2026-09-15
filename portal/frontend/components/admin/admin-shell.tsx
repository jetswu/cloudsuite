"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { Cloud, Folder, Globe, Users } from "lucide-react"
import { cn } from "cn"
import { UserMenu } from "@/components/user-menu"
import type { ReactNode } from "react"

const navItems = [
  { href: "/admin/users", label: "Users", icon: Users },
  { href: "/admin/groups", label: "Groups", icon: Folder },
  { href: "/admin/domains", label: "Domains", icon: Globe },
]

type AdminShellProps = {
  name?: string | null
  email?: string | null
  signOutAction: () => Promise<void>
  children: ReactNode
}

export function AdminShell({
  name,
  email,
  signOutAction,
  children,
}: AdminShellProps) {
  const pathname = usePathname()

  return (
    <div className="flex min-h-screen">
      {/* Sidebar */}
      <aside className="hidden w-60 shrink-0 border-r border-border bg-card md:flex md:flex-col">
        <div className="flex h-16 items-center gap-2 border-b border-border px-4">
          <span className="flex size-7 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <Cloud className="size-4" aria-hidden />
          </span>
          <span className="text-sm font-semibold tracking-tight">CloudSuite</span>
        </div>
        <nav className="flex-1 space-y-1 p-3">
          <p className="px-3 pb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Admin
          </p>
          {navItems.map(({ href, label, icon: Icon }) => {
            const active = pathname === href || pathname.startsWith(`${href}/`)
            return (
              <Link
                key={href}
                href={href}
                className={cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  active
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                )}
              >
                <Icon className="size-4" aria-hidden />
                {label}
              </Link>
            )
          })}
        </nav>
        <div className="border-t border-border p-3 text-xs text-muted-foreground">
          Area superadmin
        </div>
      </aside>

      {/* Content column */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-10 flex h-16 items-center justify-between border-b border-border bg-background/80 px-4 backdrop-blur md:px-6">
          <div className="flex items-center gap-2 md:hidden">
            <span className="flex size-7 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <Cloud className="size-4" aria-hidden />
            </span>
            <span className="text-sm font-semibold">CloudSuite</span>
          </div>
          <div className="hidden md:block" />
          <UserMenu
            name={name}
            email={email}
            isSuperAdmin
            signOutAction={signOutAction}
          />
        </header>

        <main className="flex-1 px-4 py-8 md:px-6">{children}</main>
      </div>
    </div>
  )
}
