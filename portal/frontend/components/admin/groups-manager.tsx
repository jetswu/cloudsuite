"use client"

import { useMemo, useState } from "react"
import { MoreHorizontal, Pencil, Plus, Search, Trash2, Users } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { ConfirmDialog } from "@/components/ui/confirm-dialog"
import { GroupMembersDialog } from "@/components/admin/group-members-dialog"
import { useAdminResource } from "@/hooks/use-admin-resource"
import { adminApi, type AdminGroup } from "@/lib/api"

type GroupFormState = { uuid: string | null; name: string }

export function GroupsManager({ accessToken }: { accessToken?: string }) {
  const { data, setData, loading, error, refresh } = useAdminResource<AdminGroup>(
    accessToken,
    adminApi.listGroups,
  )

  const [search, setSearch] = useState("")
  const [form, setForm] = useState<GroupFormState>({ uuid: null, name: "" })
  const [formOpen, setFormOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [membersGroup, setMembersGroup] = useState<AdminGroup | null>(null)
  const [deleting, setDeleting] = useState<AdminGroup | null>(null)
  const [busy, setBusy] = useState(false)

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (q === "") return data
    return data.filter((g) => g.name.toLowerCase().includes(q))
  }, [data, search])

  function openCreate() {
    setForm({ uuid: null, name: "" })
    setFormError(null)
    setFormOpen(true)
  }

  function openEdit(g: AdminGroup) {
    setForm({ uuid: g.pk, name: g.name })
    setFormError(null)
    setFormOpen(true)
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!accessToken) return
    setSaving(true)
    setFormError(null)
    try {
      if (form.uuid) {
        const updated = await adminApi.updateGroup(accessToken, form.uuid, {
          name: form.name,
        })
        setData((prev) => prev.map((g) => (g.pk === updated.pk ? updated : g)))
      } else {
        const created = await adminApi.createGroup(accessToken, {
          name: form.name,
        })
        setData((prev) => [...prev, created])
      }
      setFormOpen(false)
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal menyimpan group.")
    } finally {
      setSaving(false)
    }
  }

  async function confirmDelete() {
    if (!accessToken || !deleting) return
    setBusy(true)
    try {
      await adminApi.deleteGroup(accessToken, deleting.pk)
      setData((prev) => prev.filter((g) => g.pk !== deleting.pk))
      setDeleting(null)
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal menghapus group.")
    } finally {
      setBusy(false)
    }
  }

  function handleMembersChanged() {
    refresh()
  }

  return (
    <div className="space-y-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Groups</h1>
          <p className="text-sm text-muted-foreground">
            Kelola group dan izin anggota organisasi.
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="size-4" aria-hidden />
          Tambah group
        </Button>
      </header>

      <div className="relative w-full max-w-xs">
        <Search className="absolute top-2.5 left-2.5 size-4 text-muted-foreground" />
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Cari group…"
          className="pl-8"
        />
      </div>

      {formError ? (
        <p className="text-sm text-destructive">{formError}</p>
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
                  <th className="px-6 py-3 font-medium">Nama</th>
                  <th className="px-6 py-3 font-medium">Superuser</th>
                  <th className="px-6 py-3 font-medium">Anggota</th>
                  <th className="px-6 py-3 text-right font-medium">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {filtered.length === 0 ? (
                  <tr>
                    <td
                      colSpan={4}
                      className="px-6 py-8 text-center text-muted-foreground"
                    >
                      Tidak ada group.
                    </td>
                  </tr>
                ) : (
                  filtered.map((g) => (
                    <tr
                      key={g.pk}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-6 py-3 font-medium">{g.name}</td>
                      <td className="px-6 py-3">
                        {g.is_superuser ? "Ya" : "Tidak"}
                      </td>
                      <td className="px-6 py-3 text-muted-foreground">
                        {(g.users ?? []).length}
                      </td>
                      <td className="px-6 py-3 text-right">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              aria-label={`Aksi ${g.name}`}
                            >
                              <MoreHorizontal className="size-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              onClick={() => setMembersGroup(g)}
                            >
                              <Users className="size-4" />
                              Kelola member
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => openEdit(g)}>
                              <Pencil className="size-4" />
                              Edit
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              variant="destructive"
                              onClick={() => setDeleting(g)}
                            >
                              <Trash2 className="size-4" />
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

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>
              {form.uuid ? "Edit group" : "Tambah group"}
            </DialogTitle>
            <DialogDescription>
              {form.uuid
                ? "Ubah nama group."
                : "Buat group baru untuk organisasi."}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="group-name">Nama group</Label>
              <Input
                id="group-name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                required
              />
            </div>
            {formError ? (
              <p className="text-sm text-destructive">{formError}</p>
            ) : null}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setFormOpen(false)}
                disabled={saving}
              >
                Batal
              </Button>
              <Button type="submit" disabled={saving}>
                {form.uuid ? "Simpan" : "Tambah"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <GroupMembersDialog
        open={membersGroup !== null}
        onOpenChange={(open) => {
          if (!open) setMembersGroup(null)
        }}
        token={accessToken}
        group={membersGroup ? { pk: membersGroup.pk, name: membersGroup.name } : null}
        onChanged={handleMembersChanged}
      />

      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title="Hapus group?"
        description={
          deleting
            ? `Group "${deleting.name}" akan dihapus permanen.`
            : undefined
        }
        confirmLabel="Hapus"
        loading={busy}
        onConfirm={confirmDelete}
      />
    </div>
  )
}
