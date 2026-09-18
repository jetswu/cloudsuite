"use client"

// Reusable shell for the Sprint 1.5a dashboard widgets: header (icon, title,
// badge, refresh, deep link), loading skeleton, and the shared
// "Data tidak tersedia" / retry states. The body is per-widget.

import type { ReactNode } from "react"
import { ArrowUpRight, RefreshCw } from "lucide-react"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "cn"

type WidgetShellProps = {
  icon: React.ComponentType<{ className?: string; "aria-hidden"?: boolean }>
  title: string
  badge?: ReactNode
  href?: string // "buka layanan" link in the header
  loading?: boolean
  error?: string | null
  unavailableText?: string // shown instead of the body when the service is not provisioned
  onRefresh: () => void
  children: ReactNode
}

export function WidgetShell({
  icon: Icon,
  title,
  badge,
  href,
  loading,
  error,
  unavailableText,
  onRefresh,
  children,
}: WidgetShellProps) {
  const unavailable = Boolean(unavailableText)
  const showBody = !loading && !error && !unavailable

  return (
    <div className="flex flex-col rounded-lg border border-border bg-card p-5">
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="flex size-8 items-center justify-center rounded-md border border-border bg-background text-foreground">
            <Icon className="size-4" aria-hidden />
          </span>
          <h3 className="text-sm font-semibold">{title}</h3>
          {badge}
        </div>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={onRefresh}
            aria-label={`Refresh ${title}`}
            className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <RefreshCw className={cn("size-3.5", loading && "animate-spin")} aria-hidden />
          </button>
          {href && (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={`Buka ${title}`}
              className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
            >
              <ArrowUpRight className="size-4" aria-hidden />
            </a>
          )}
        </div>
      </div>

      <div className="mt-4 flex-1">
        {loading ? (
          <div className="space-y-2.5">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-5/6" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        ) : error ? (
          <div className="flex h-full flex-col items-start justify-center gap-2">
            <p className="text-sm text-muted-foreground">Data tidak tersedia.</p>
            <button
              type="button"
              onClick={onRefresh}
              className="text-sm font-medium text-primary underline-offset-4 hover:underline"
            >
              Coba lagi
            </button>
          </div>
        ) : unavailable ? (
          <p className="text-sm text-muted-foreground">{unavailableText}</p>
        ) : (
          children
        )}
      </div>
    </div>
  )
}

export function formatBytes(n: number): string {
  if (n < 0) return "—"
  if (n < 1024) return `${n} B`
  const units = ["KB", "MB", "GB", "TB"]
  let v = n
  let i = -1
  do {
    v /= 1024
    i++
  } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(v >= 100 ? 0 : 1)} ${units[i]}`
}

export function formatRelative(iso: string): string {
  const d = new Date(iso)
  const diff = Date.now() - d.getTime()
  const min = Math.round(diff / 60000)
  if (min < 1) return "baru saja"
  if (min < 60) return `${min} mnt lalu`
  const h = Math.round(min / 60)
  if (h < 24) return `${h} jam lalu`
  const days = Math.round(h / 24)
  if (days < 7) return `${days} hari lalu`
  return d.toLocaleDateString("id-ID", { day: "numeric", month: "short" })
}
