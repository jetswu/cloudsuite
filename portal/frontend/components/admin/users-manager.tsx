"use client"

import { useMemo, useState } from "react"
import { Ban, Check, MoreHorizontal, Pencil, Plus, Search } from "lucide-react"
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
import { adminApi, type AdminUser } from "@/lib/api"

type StatusFilter = "all" | "active" | "suspended"

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
  const { data, setData, loading, error, refresh } = useAdminResource<AdminUser>(
    accessToken,
    adminApi.listUsers,
  )
  const groups = useGroups(accessToken)

  const [search, setSearch] = useState("")
  const [status, setStatus] = useState<StatusFilter>("all")
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<AdminUser | null>(null)
  const [deleting, setDeleting] = useState<AdminUser | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [createdUser, setCreatedUser] = useState<AdminUser | null>(null)
  const [createdPassword, setCreatedPassword] = useState<string | null>(null)

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

  function openEdit(u: AdminUser) {
    setEditing(u)
    setFormOpen(true)
  }

  function handleSaved(user: AdminUser, password?: string) {
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

  async function toggleActive(u: AdminUser) {
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
      await adminApi.deleteUser(accessToken, deleting.pk)
      setData((prev) => prev.filter((u) => u.pk !== deleting.pk))
      setDeleting(null)
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Gagal menghapus user.",
      )
    } finally {
      setBusy(false)
    }
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
                  <th className="px-6 py-3 text-right font-medium">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {filtered.length === 0 ? (
                  <tr>
                    <td
                      colSpan={6}
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
