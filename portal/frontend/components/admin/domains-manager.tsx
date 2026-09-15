"use client"

import { useState } from "react"
import Link from "next/link"
import {
  ArrowRight,
  CheckCircle2,
  Clock,
  Plus,
  ShieldCheck,
  Trash2,
  XCircle,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useDomains } from "@/hooks/use-domains"
import { adminApi, type PortalDomain } from "@/lib/api"

function statusLabel(status: PortalDomain["status"]): string {
  switch (status) {
    case "pending":
      return "Menunggu"
    case "dns_in_progress":
      return "DNS sedang diproses"
    case "verified":
      return "Terverifikasi"
    case "active":
      return "Aktif"
    case "error":
      return "Gagal"
    default:
      return status
  }
}

function StatusBadge({ status }: { status: PortalDomain["status"] }) {
  const tone =
    status === "verified" || status === "active"
      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
      : status === "error"
        ? "bg-destructive/10 text-destructive"
        : status === "dns_in_progress"
          ? "bg-amber-500/10 text-amber-600 dark:text-amber-400"
          : "bg-muted text-muted-foreground"

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${tone}`}
    >
      {status === "verified" || status === "active" ? (
        <CheckCircle2 className="size-3.5" />
      ) : status === "error" ? (
        <XCircle className="size-3.5" />
      ) : status === "dns_in_progress" ? (
        <Clock className="size-3.5" />
      ) : (
        <ShieldCheck className="size-3.5" />
      )}
      {statusLabel(status)}
    </span>
  )
}

export function DomainsManager({ accessToken }: { accessToken?: string }) {
  const { data, setData, loading, error, refresh } = useDomains(accessToken)

  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState("")
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<PortalDomain | null>(null)

  async function handleCreate() {
    if (!accessToken) return
    setBusy(true)
    setActionError(null)
    try {
      const detail = await adminApi.createDomain(accessToken, { name })
      setData((prev) => [detail.domain, ...prev])
      setCreateOpen(false)
      setName("")
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Gagal membuat domain.",
      )
    } finally {
      setBusy(false)
    }
  }

  async function confirmDelete() {
    if (!accessToken || !deleting) return
    setBusy(true)
    setActionError(null)
    try {
      await adminApi.deleteDomain(accessToken, deleting.id)
      setData((prev) => prev.filter((d) => d.id !== deleting.id))
      setDeleting(null)
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Gagal menghapus domain.",
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Domains</h1>
          <p className="text-sm text-muted-foreground">
            Tambahkan domain pelanggan dan siapkan DNS-nya (MX, SPF, DKIM,
            DMARC).
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="size-4" aria-hidden />
          Tambah Domain
        </Button>
      </header>

      {actionError ? (
        <p className="text-sm text-destructive">{actionError}</p>
      ) : null}

      <div className="overflow-hidden rounded-xl border border-border">
        {loading ? (
          <div className="space-y-2 p-6">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : error ? (
          <div className="flex items-center justify-between p-6">
            <p className="text-sm text-destructive">{error}</p>
            <Button variant="outline" size="sm" onClick={refresh}>
              Coba lagi
            </Button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-border text-left text-xs uppercase tracking-wider text-muted-foreground">
                <tr>
                  <th className="px-6 py-3 font-medium">Domain</th>
                  <th className="px-6 py-3 font-medium">Status</th>
                  <th className="px-6 py-3 font-medium">Dibuat</th>
                  <th className="px-6 py-3 text-right font-medium">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {data.length === 0 ? (
                  <tr>
                    <td
                      colSpan={4}
                      className="px-6 py-10 text-center text-muted-foreground"
                    >
                      Belum ada domain. Klik &ldquo;Tambah Domain&rdquo; untuk
                      memulai.
                    </td>
                  </tr>
                ) : (
                  data.map((d) => (
                    <tr
                      key={d.id}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-6 py-3 font-medium">{d.name}</td>
                      <td className="px-6 py-3">
                        <StatusBadge status={d.status} />
                      </td>
                      <td className="px-6 py-3 text-muted-foreground">
                        {new Date(d.created_at).toLocaleDateString("id-ID", {
                          day: "2-digit",
                          month: "short",
                          year: "numeric",
                        })}
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            asChild
                          >
                            <Link href={`/admin/domains/${d.id}/setup`}>
                              <ArrowRight className="size-4" aria-hidden />
                              Setup
                            </Link>
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={`Hapus ${d.name}`}
                            onClick={() => setDeleting(d)}
                            disabled={busy}
                          >
                            <Trash2 className="size-4 text-destructive" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Tambah Domain</DialogTitle>
            <DialogDescription>
              Masukkan nama domain pelanggan. DNS record (MX, SPF, DKIM, DMARC)
              akan dibuat otomatis.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="contoh: sman1.sch.id"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter") void handleCreate()
              }}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setCreateOpen(false)}
              disabled={busy}
            >
              Batal
            </Button>
            <Button onClick={handleCreate} disabled={busy || name.trim() === ""}>
              {busy ? "Memproses…" : "Buat"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title="Hapus domain?"
        description={
          deleting
            ? `Domain "${deleting.name}" akan dihapus dari Portal dan Stalwart. Tindakan ini tidak bisa dibatalkan.`
            : undefined
        }
        confirmLabel="Hapus"
        loading={busy}
        onConfirm={confirmDelete}
      />
    </div>
  )
}
