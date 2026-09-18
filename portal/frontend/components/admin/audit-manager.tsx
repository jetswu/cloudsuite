"use client"

// Sprint 1.5a: /admin/audit — read-only view over the append-only audit_logs
// table: filter bar (date range, actor, action, service, search), pagination,
// CSV export, and a per-row detail modal with the metadata JSON.

import { useCallback, useEffect, useMemo, useState } from "react"
import { Download, RefreshCw, Search } from "lucide-react"
import { adminApi, type AuditEntry, type AuditFilters } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "cn"

const PAGE_SIZE = 50

const ACTION_OPTIONS = [
  "user.create",
  "user.update",
  "user.delete",
  "user.set_groups",
  "group.create",
  "group.update",
  "group.delete",
  "domain.create",
  "domain.update_dkim",
  "domain.delete",
  "domain.verify",
  "provisioning.retry",
]

const SERVICE_OPTIONS = [
  { value: "mail", label: "Mail" },
  { value: "drive", label: "Drive" },
  { value: "erp", label: "ERP" },
  { value: "none", label: "Tanpa service" },
]

type ActorOption = { pk: number; email: string }

type Props = { accessToken?: string }

export function AuditManager({ accessToken }: Props) {
  const [from, setFrom] = useState("")
  const [to, setTo] = useState("")
  const [actorId, setActorId] = useState("")
  const [action, setAction] = useState("")
  const [service, setService] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(1)

  const [items, setItems] = useState<AuditEntry[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [exporting, setExporting] = useState(false)

  const [actors, setActors] = useState<ActorOption[]>([])
  const [selected, setSelected] = useState<AuditEntry | null>(null)

  // Debounce the free-text search so typing does not hammer the API.
  useEffect(() => {
    const t = setTimeout(() => {
      setSearch(searchInput.trim())
      setPage(1)
    }, 400)
    return () => clearTimeout(t)
  }, [searchInput])

  const filters: AuditFilters = useMemo(
    () => ({
      from: from || undefined,
      to: to || undefined,
      actor_id: actorId || undefined,
      action: action || undefined,
      service: service || undefined,
      search: search || undefined,
    }),
    [from, to, actorId, action, service, search],
  )

  const load = useCallback(async () => {
    if (!accessToken) return
    setLoading(true)
    setError(null)
    try {
      const res = await adminApi.listAuditLogs(accessToken, { ...filters, page })
      setItems(res.items)
      setTotal(res.total)
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat audit log")
    } finally {
      setLoading(false)
    }
  }, [accessToken, filters, page])

  useEffect(() => {
    load()
  }, [load])

  // Actor dropdown options come from the admin users list.
  useEffect(() => {
    if (!accessToken) return
    adminApi
      .listUsers(accessToken)
      .then((users) =>
        setActors(
          users
            .filter((u) => u.email)
            .map((u) => ({ pk: u.pk, email: u.email })),
        ),
      )
      .catch(() => setActors([]))
  }, [accessToken])

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  async function exportCSV() {
    if (!accessToken || exporting) return
    setExporting(true)
    try {
      await adminApi.exportAuditLogs(accessToken, filters)
    } catch {
      // Download failure is surfaced transiently via the normal error row.
      setError("Export CSV gagal")
    } finally {
      setExporting(false)
    }
  }

  function resetFilters() {
    setFrom("")
    setTo("")
    setActorId("")
    setAction("")
    setService("")
    setSearchInput("")
    setSearch("")
    setPage(1)
  }

  const hasFilters =
    from || to || actorId || action || service || search ? true : false

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Audit Log</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Jejak aktivitas admin. Log bersifat immutable — tidak bisa diedit
            atau dihapus.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={load} disabled={loading}>
            <RefreshCw
              className={cn("size-4", loading && "animate-spin")}
              aria-hidden
            />
            Refresh
          </Button>
          <Button size="sm" onClick={exportCSV} disabled={exporting || loading}>
            <Download className="size-4" aria-hidden />
            {exporting ? "Mengekspor…" : "Export CSV"}
          </Button>
        </div>
      </div>

      {/* Filter bar */}
      <div className="grid gap-3 rounded-lg border border-border bg-card p-4 sm:grid-cols-2 lg:grid-cols-6">
        <label className="space-y-1 text-xs font-medium text-muted-foreground">
          Dari
          <Input
            type="date"
            value={from}
            onChange={(e) => {
              setFrom(e.target.value)
              setPage(1)
            }}
          />
        </label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground">
          Sampai
          <Input
            type="date"
            value={to}
            onChange={(e) => {
              setTo(e.target.value)
              setPage(1)
            }}
          />
        </label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground">
          Actor
          <select
            value={actorId}
            onChange={(e) => {
              setActorId(e.target.value)
              setPage(1)
            }}
            className="h-9 w-full rounded-md border border-input bg-background px-2 text-sm"
          >
            <option value="">Semua</option>
            {actors.map((a) => (
              <option key={a.pk} value={a.pk}>
                {a.email}
              </option>
            ))}
          </select>
        </label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground">
          Aksi
          <select
            value={action}
            onChange={(e) => {
              setAction(e.target.value)
              setPage(1)
            }}
            className="h-9 w-full rounded-md border border-input bg-background px-2 text-sm"
          >
            <option value="">Semua</option>
            {ACTION_OPTIONS.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground">
          Service
          <select
            value={service}
            onChange={(e) => {
              setService(e.target.value)
              setPage(1)
            }}
            className="h-9 w-full rounded-md border border-input bg-background px-2 text-sm"
          >
            <option value="">Semua</option>
            {SERVICE_OPTIONS.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </label>
        <label className="space-y-1 text-xs font-medium text-muted-foreground">
          Cari
          <div className="relative">
            <Search
              className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden
            />
            <Input
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="email, target, metadata…"
              className="pl-8"
            />
          </div>
        </label>
        {hasFilters && (
          <button
            type="button"
            onClick={resetFilters}
            className="text-left text-xs font-medium text-primary underline-offset-4 hover:underline lg:col-span-6"
          >
            Reset semua filter
          </button>
        )}
      </div>

      {/* Table */}
      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead className="bg-muted/50 text-left text-xs uppercase tracking-wider text-muted-foreground">
            <tr>
              <th className="px-4 py-3 font-medium">Waktu</th>
              <th className="px-4 py-3 font-medium">Actor</th>
              <th className="px-4 py-3 font-medium">Aksi</th>
              <th className="px-4 py-3 font-medium">Target</th>
              <th className="px-4 py-3 font-medium">Service</th>
              <th className="px-4 py-3 font-medium">IP</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i}>
                  <td colSpan={6} className="px-4 py-3">
                    <Skeleton className="h-4 w-full" />
                  </td>
                </tr>
              ))
            ) : error ? (
              <tr>
                <td colSpan={6} className="px-4 py-6 text-center">
                  <p className="text-sm text-muted-foreground">{error}</p>
                  <button
                    type="button"
                    onClick={load}
                    className="mt-2 text-sm font-medium text-primary underline-offset-4 hover:underline"
                  >
                    Coba lagi
                  </button>
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td
                  colSpan={6}
                  className="px-4 py-6 text-center text-sm text-muted-foreground"
                >
                  Tidak ada log yang cocok.
                </td>
              </tr>
            ) : (
              items.map((a) => (
                <tr
                  key={a.id}
                  onClick={() => setSelected(a)}
                  className="cursor-pointer transition-colors hover:bg-accent"
                >
                  <td className="whitespace-nowrap px-4 py-3 text-muted-foreground">
                    {formatTime(a.created_at)}
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-medium">{a.actor_email || "—"}</span>
                    {a.actor_type !== "user" && (
                      <span className="ml-1.5 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                        {a.actor_type}
                      </span>
                    )}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-xs">
                    {a.action}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {a.target_type ? `${a.target_type} ${a.target_id ?? ""}` : "—"}
                  </td>
                  <td className="px-4 py-3">{a.service_code ?? "—"}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-muted-foreground">
                    {a.ip_address ?? "—"}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          {total} log · halaman {page} dari {totalPages}
        </span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1 || loading}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            Sebelumnya
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages || loading}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          >
            Berikutnya
          </Button>
        </div>
      </div>

      {selected && (
        <AuditDetailModal
          entry={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  )
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  })
}

function AuditDetailModal({
  entry,
  onClose,
}: {
  entry: AuditEntry
  onClose: () => void
}) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose()
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [onClose])

  const rows: [string, string][] = [
    ["ID", String(entry.id)],
    ["Waktu", formatTime(entry.created_at)],
    ["Actor", `${entry.actor_email || "—"} (pk ${entry.actor_id ?? "—"}, ${entry.actor_type})`],
    ["Aksi", entry.action],
    [
      "Target",
      entry.target_type ? `${entry.target_type} ${entry.target_id ?? ""}` : "—",
    ],
    ["Service", entry.service_code ?? "—"],
    ["IP", entry.ip_address ?? "—"],
    ["User-Agent", entry.user_agent ?? "—"],
  ]

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm"
      onClick={onClose}
      role="presentation"
    >
      <div
        className="max-h-[85vh] w-full max-w-xl overflow-y-auto rounded-lg border border-border bg-card p-6 shadow-lg"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Detail audit log"
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Detail log #{entry.id}</h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-accent"
          >
            Tutup
          </button>
        </div>
        <dl className="space-y-2 text-sm">
          {rows.map(([k, v]) => (
            <div key={k} className="flex gap-2">
              <dt className="w-24 shrink-0 text-muted-foreground">{k}</dt>
              <dd className="min-w-0 break-words font-medium">{v}</dd>
            </div>
          ))}
        </dl>
        <p className="mb-1 mt-4 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Metadata
        </p>
        <pre className="overflow-x-auto rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
          {JSON.stringify(entry.metadata ?? {}, null, 2)}
        </pre>
      </div>
    </div>
  )
}
