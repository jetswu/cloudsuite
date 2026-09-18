"use client"

// Drive widget: storage usage bar + 5 most recently modified files
// (Nextcloud OCS + shared-postgres filecache via the portal backend).

import { useCallback, useEffect, useState } from "react"
import { HardDrive } from "lucide-react"
import { widgetApi, type DriveWidgetSummary } from "@/lib/api"
import { WidgetShell, formatBytes, formatRelative } from "@/components/dashboard/widget-shell"

const REFRESH_MS = 5 * 60 * 1000

export function WidgetDrive({ accessToken }: { accessToken?: string }) {
  const [data, setData] = useState<DriveWidgetSummary | null>(null)
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
      setData(await widgetApi.getDriveWidget(accessToken))
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

  const unlimited = Boolean(data && data.storage.total_bytes <= 0)
  const percent = data && data.storage.percent > 100 ? 100 : data?.storage.percent ?? 0

  return (
    <WidgetShell
      icon={HardDrive}
      title="Drive"
      href="https://drive.idchsuite.my.id"
      loading={loading}
      error={error}
      unavailableText={
        data && !data.unavailable && !data.provisioned
          ? "Drive belum diaktifkan untuk akun ini."
          : undefined
      }
      onRefresh={load}
    >
      {data && (
        <div className="space-y-1.5">
          <div className="flex items-baseline justify-between text-sm">
            <span className="font-medium">{formatBytes(data.storage.used_bytes)}</span>
            <span className="text-xs text-muted-foreground">
              {unlimited ? "tanpa batas" : `dari ${formatBytes(data.storage.total_bytes)}`}
            </span>
          </div>
          <div
            className="h-2 overflow-hidden rounded-full bg-muted"
            role="progressbar"
            aria-valuenow={unlimited ? undefined : Math.round(percent)}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="Pemakaian penyimpanan"
          >
            <div
              className="h-full rounded-full bg-primary transition-all"
              style={{ width: `${unlimited ? 100 : percent}%` }}
            />
          </div>
        </div>
      )}

      {data && data.recent.length === 0 ? (
        <p className="mt-3 text-sm text-muted-foreground">Belum ada file.</p>
      ) : (
        <ul className="mt-3 space-y-2">
          {data?.recent.map((f, i) => (
            <li key={`${f.name}-${i}`}>
              <a
                href={f.deep_link}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-baseline justify-between gap-2 rounded-md px-2 py-1.5 transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
              >
                <span className="truncate text-sm">{f.name}</span>
                <span className="shrink-0 text-xs text-muted-foreground">
                  {formatBytes(f.size)} · {formatRelative(f.modified_at)}
                </span>
              </a>
            </li>
          ))}
        </ul>
      )}
    </WidgetShell>
  )
}
