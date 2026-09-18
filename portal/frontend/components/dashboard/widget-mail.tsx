"use client"

// Mail widget: unread count badge + 5 most recent messages (Stalwart JMAP
// via the portal backend, cached 5 minutes server-side).

import { useCallback, useEffect, useState } from "react"
import { Mail } from "lucide-react"
import { widgetApi, type MailWidgetSummary } from "@/lib/api"
import { WidgetShell, formatRelative } from "@/components/dashboard/widget-shell"

const REFRESH_MS = 5 * 60 * 1000

export function WidgetMail({ accessToken }: { accessToken?: string }) {
  const [data, setData] = useState<MailWidgetSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!accessToken) {
      setError("Tidak ada token")
      setLoading(false)
      return
    }
    setLoading(true)
    try {
      setData(await widgetApi.getMailWidget(accessToken))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : "gagal memuat")
    } finally {
      setLoading(false)
    }
  }, [accessToken])

  useEffect(() => {
    load()
    const t = setInterval(load, REFRESH_MS)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accessToken])

  const badge =
    data && !data.unavailable && data.unread_count > 0 ? (
      <span className="rounded-full bg-primary px-2 py-0.5 text-xs font-semibold text-primary-foreground">
        {data.unread_count} belum dibaca
      </span>
    ) : null

  return (
    <WidgetShell
      icon={Mail}
      title="Mail"
      badge={badge}
      href="https://webmail.idchsuite.my.id"
      loading={loading}
      error={error}
      unavailableText={
        data && !data.unavailable && !data.provisioned
          ? "Mail belum diaktifkan untuk akun ini."
          : undefined
      }
      onRefresh={load}
    >
      {data && data.recent.length === 0 ? (
        <p className="text-sm text-muted-foreground">Belum ada pesan masuk.</p>
      ) : (
        <ul className="space-y-2.5">
          {data?.recent.map((m) => (
            <li key={m.id}>
              <a
                href={m.deep_link}
                target="_blank"
                rel="noopener noreferrer"
                className="block rounded-md px-2 py-1.5 transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
              >
                <div className="flex items-baseline justify-between gap-2">
                  <span className="truncate text-sm font-medium">
                    {m.from || "(tanpa pengirim)"}
                  </span>
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {formatRelative(m.received_at)}
                  </span>
                </div>
                <div className="flex items-center gap-1.5">
                  {!m.is_read && (
                    <span
                      className="size-1.5 shrink-0 rounded-full bg-primary"
                      aria-label="belum dibaca"
                    />
                  )}
                  <span className="truncate text-xs text-muted-foreground">
                    {m.subject || "(tanpa subjek)"}
                  </span>
                </div>
              </a>
            </li>
          ))}
        </ul>
      )}
    </WidgetShell>
  )
}
