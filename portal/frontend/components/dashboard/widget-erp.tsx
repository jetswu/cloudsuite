"use client"

// ERP widget: metric cards from the Odoo JSON-RPC summary (cached 15 minutes
// server-side). The base install has no Accounting/CRM data, so the metrics
// are the ones the deployment actually has (partners, mail activity, users).

import { useCallback, useEffect, useState } from "react"
import { BarChart3 } from "lucide-react"
import { widgetApi, type ErpWidgetSummary } from "@/lib/api"
import { WidgetShell } from "@/components/dashboard/widget-shell"

const REFRESH_MS = 15 * 60 * 1000

export function WidgetErp({ accessToken }: { accessToken?: string }) {
  const [data, setData] = useState<ErpWidgetSummary | null>(null)
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
      setData(await widgetApi.getErpWidget(accessToken))
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

  return (
    <WidgetShell
      icon={BarChart3}
      title="ERP"
      href="https://erp.idchsuite.my.id"
      loading={loading}
      error={error}
      onRefresh={load}
    >
      <div className="grid grid-cols-3 gap-2">
        {data?.metrics.map((m) => (
          <div
            key={m.key}
            className="rounded-md border border-border bg-background p-2.5 text-center"
          >
            <p className="text-xl font-semibold tracking-tight">{m.value}</p>
            <p className="mt-0.5 text-[11px] leading-tight text-muted-foreground">
              {m.label}
            </p>
          </div>
        ))}
      </div>
    </WidgetShell>
  )
}
