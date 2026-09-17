"use client"

import { useMemo, useState, useEffect } from "react"
import {
  Ban,
  Check,
  MoreHorizontal,
  Pencil,
  Plus,
  RotateCcw,
  Search,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { UserFormDialog } from "@/components/admin/user-form-dialog"
import { UserCreatedDialog } from "@/components/admin/user-created-dialog"
import { useAdminResource } from "@/hooks/use-admin-resource"
import { useGroups } from "@/hooks/use-groups"
import { adminApi, type AdminUserWithProvisioning } from "@/lib/api"

type StatusFilter = "all" | "active" | "suspended"

// Sprint 1.4b: provisioning badge rendering per service status.
const PROV_BADGE: Record<string, { label: string; cls: string }> = {
  active: { label: "✅", cls: "bg-emerald-500/15 text-emerald-600" },
  provisioning: { label: "⏳", cls: "bg-amber-500/15 text-amber-600" },
  pending: { label: "⏳", cls: "bg-amber-500/15 text-amber-600" },
  failed: { label: "⚠️", cls: "bg-destructive/15 text-destructive" },
  pending_delete: { label: "⛔", cls: "bg-orange-500/15 text-orange-600" },
  deleted: { label: "⛔", cls: "bg-muted text-muted-foreground" },
}

function ProvBadge({ service, status }: { service: string; status: string }) {
  const b = PROV_BADGE[status] ?? {
    label: "·",
    cls: "bg-muted text-muted-foreground",
  }
  return (
    <span
      title={`${service}: ${status}`}
      className={`inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-medium ${b.cls}`}
    >
      <span aria-hidden>{b.label}</span>
      <span className="uppercase">{service.slice(0, 4)}</span>
    </span>
  )
}

function formatDateTime(value: string | null): string {
  if (!value) return "—"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return "—"
  return d.toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function UsersManager({ accessToken }: { accessToken?: string }) {
  const { data, setData, loading, error, refresh } =
    useAdminResource<AdminUserWithProvisioning>(accessToken, adminApi.listUsers)
  const groups = useGroups(accessToken)

  const [search, setSearch] = useState("")
  const [status, setStatus] = useState<StatusFilter>("all")
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<AdminUserWithProvisioning | null>(null)
  const [deleting, setDeleting] = useState<AdminUserWithProvisioning | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [createdUser, setCreatedUser] = useState<AdminUserWithProvisioning | null>(null)
  const [createdPassword, setCreatedPassword] = useState<string | null>(null)

  // Sprint 1.4b: poll every 5s while any visible user has jobs in flight so
  // provisioning badges resolve without a manual refresh. Stops when idle.
  const hasPending = useMemo(
    () =>
      data.some((u) =>
        ["pending", "provisioning", "pending_delete"].some((s) =>
          [u.provisioning?.stalwart, u.provisioning?.nextcloud, u.provisioning?.odoo].includes(
            s as never,
          ),
        ),
      ),
    [data],
  )
  useEffect(() => {
    if (!hasPending) return
    const t = setInterval(refresh, 5000)
    return () => clearInterval(t)
  }, [hasPending, refresh])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return data.filter((u) => {
      if (status === "active" && !u.is_active) return false
      if (status === "suspended" && u.is_active) return false
      if (q === "") return true
      return (
        u.username.toLowerCase().includes(q) ||
        u.name.toLowerCase().includes(q) ||
        u.email.toLowerCase().includes(q)
      )
    })
  }, [data, search, status])

  function openCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function openEdit(u: AdminUserWithProvisioning) {
    setEditing(u)
    setFormOpen(true)
  }

  function handleSaved(user: AdminUserWithProvisioning, password?: string) {
    setData((prev) => {
      const exists = prev.some((u) => u.pk === user.pk)
      return exists
        ? prev.map((u) => (u.pk === user.pk ? user : u))
        : [...prev, user]
    })
    if (password !== undefined) {
      setCreatedUser(user)
      setCreatedPassword(password)
    }
  }

  function closeCreatedDialog() {
    setCreatedUser(null)
    setCreatedPassword(null)
  }

  async function toggleActive(u: AdminUserWithProvisioning) {
    if (!accessToken) return
    setBusy(true)
    setActionError(null)
    try {
      const updated = await adminApi.updateUser(accessToken, u.pk, {
        username: u.username,
        name: u.name,
        email: u.email,
        is_active: !u.is_active,
      })
      setData((prev) => prev.map((x) => (x.pk === u.pk ? updated : x)))
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Gagal mengubah status.",
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
      const res = await adminApi.deleteUser(accessToken, deleting.pk)
      if (!res.deprovision_queued) {
        setActionError(
          "User terhapus, tapi de-provisioning tidak ter-enqueue (tidak ada state provisioning).",
        )
      }
      setDeleting(null)
      await refresh()
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Gagal menghapus user.",
      )
    } finally {
      setBusy(false)
    }
  }

  // Sprint 1.4b: retry the latest failed provisioning job per service.
  async function retryProvisioning(u: AdminUserWithProvisioning, service: string) {
    if (!accessToken) return
    setBusy(true)
    setActionError(null)
    try {
      await adminApi.retryProvisioning(accessToken, u.pk, service)
      await refresh()
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Gagal retry provisioning.",
      )
    } finally {
      setBusy(false)
    }
  }

  function hasFailedJob(u: AdminUserWithProvisioning): boolean {
    const p = u.provisioning
    return [p?.stalwart, p?.nextcloud, p?.odoo].includes("failed")
  }

  return (
    <div className="space-y-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Users</h1>
          <p className="text-sm text-muted-foreground">
            Kelola akun pengguna organisasi.
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="size-4" aria-hidden />
          Tambah user
        </Button>
      </header>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full max-w-xs">
          <Search className="absolute top-2.5 left-2.5 size-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Cari username, nama, email…"
            className="pl-8"
          />
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-border p-1">
          {(
            [
              ["all", "Semua"],
              ["active", "Aktif"],
              ["suspended", "Ditangguhkan"],
            ] as [StatusFilter, string][]
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              onClick={() => setStatus(value)}
              className={
                status === value
                  ? "rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground"
                  : "rounded-md px-3 py-1.5 text-xs font-medium text-muted-foreground hover:bg-accent"
              }
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {actionError ? (
        <p className="text-sm text-destructive">{actionError}</p>
      ) : null}

      <div className="overflow-hidden rounded-xl border border-border">
        {loading ? (
          <div className="space-y-2 p-6">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
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
                  <th className="px-6 py-3 font-medium">Username</th>
                  <th className="px-6 py-3 font-medium">Nama</th>
                  <th className="px-6 py-3 font-medium">Group</th>
                  <th className="px-6 py-3 font-medium">Last Login</th>
                  <th className="px-6 py-3 font-medium">Status</th>
                  <th className="px-6 py-3 font-medium">Provisioning</th>
                  <th className="px-6 py-3 text-right font-medium">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {filtered.length === 0 ? (
                  <tr>
                    <td
                      colSpan={7}
                      className="px-6 py-8 text-center text-muted-foreground"
                    >
                      Tidak ada user.
                    </td>
                  </tr>
                ) : (
                  filtered.map((u) => (
                    <tr
                      key={u.pk}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-6 py-3 font-medium">{u.username}</td>
                      <td className="px-6 py-3">{u.name || "—"}</td>
                      <td className="px-6 py-3">
                        <div className="flex flex-wrap gap-1">
                          {(u.groups_obj ?? []).length === 0 ? (
                            <span className="text-xs text-muted-foreground">
                              —
                            </span>
                          ) : (
                            (u.groups_obj ?? []).map((g) => (
                              <span
                                key={g.pk}
                                className="inline-flex items-center rounded-md border border-border bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                              >
                                {g.name}
                              </span>
                            ))
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-3 text-muted-foreground">
                        {formatDateTime(u.last_login)}
                      </td>
                      <td className="px-6 py-3">
                        {u.is_active ? (
                          <span className="inline-flex items-center gap-1 text-xs font-medium text-foreground">
                            <span className="size-1.5 rounded-full bg-emerald-500" />
                            Aktif
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
                            <span className="size-1.5 rounded-full bg-destructive" />
                            Ditangguhkan
                          </span>
                        )}
                      </td>
                      <td className="px-6 py-3">
                        {u.provisioning ? (
                          <div className="flex flex-wrap gap-1">
                            <ProvBadge service="mail" status={u.provisioning.stalwart} />
                            <ProvBadge service="drive" status={u.provisioning.nextcloud} />
                            <ProvBadge service="erp" status={u.provisioning.odoo} />
                          </div>
                        ) : (
                          <span className="text-xs text-muted-foreground">—</span>
                        )}
                      </td>
                      <td className="px-6 py-3 text-right">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              aria-label={`Aksi ${u.username}`}
                            >
                              <MoreHorizontal className="size-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              onClick={() => openEdit(u)}
                              disabled={busy}
                            >
                              <Pencil className="size-4" />
                              Edit
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => toggleActive(u)}
                              disabled={busy}
                            >
                              {u.is_active ? (
                                <>
                                  <Ban className="size-4" />
                                  Suspend
                                </>
                              ) : (
                                <>
                                  <Check className="size-4" />
                                  Aktifkan
                                </>
                              )}
                            </DropdownMenuItem>
                            {hasFailedJob(u) ? (
                              <DropdownMenuItem
                                onClick={() => retryProvisioning(u, "all")}
                                disabled={busy}
                              >
                                <RotateCcw className="size-4" />
                                Retry provisioning
                              </DropdownMenuItem>
                            ) : null}
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              variant="destructive"
                              onClick={() => setDeleting(u)}
                              disabled={busy}
                            >
                              Hapus
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <UserFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        token={accessToken}
        user={editing}
        groups={groups.data}
        onSaved={handleSaved}
      />

      <UserCreatedDialog
        user={createdUser}
        password={createdPassword}
        open={createdUser !== null}
        onOpenChange={(open) => {
          if (!open) closeCreatedDialog()
        }}
      />

      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title="Hapus user?"
        description={
          deleting
            ? `Akun "${deleting.username}" akan dihapus permanen. Tindakan ini tidak bisa dibatalkan.`
            : undefined
        }
        confirmLabel="Hapus"
        loading={busy}
        onConfirm={confirmDelete}
      />
    </div>
  )
}
