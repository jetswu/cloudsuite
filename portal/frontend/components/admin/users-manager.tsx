"use client"

import { useState } from "react"
import { Plus, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { adminApi, type AdminUser, type UserRequest } from "@/lib/api"
import { useAdminResource } from "@/hooks/use-admin-resource"

const emptyForm: UserRequest = { username: "", name: "", email: "" }

export function UsersManager({ accessToken }: { accessToken?: string }) {
  const { data, setData, loading, error, refresh } = useAdminResource<AdminUser>(
    accessToken,
    adminApi.listUsers,
  )
  const [form, setForm] = useState<UserRequest>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError(null)
    try {
      const created = await adminApi.createUser(accessToken!, form)
      setData([...data, created])
      setForm(emptyForm)
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal membuat user.")
    } finally {
      setSaving(false)
    }
  }

  async function remove(id: number) {
    if (!confirm("Hapus user ini?")) return
    try {
      await adminApi.deleteUser(accessToken!, id)
      setData(data.filter((u) => u.pk !== id))
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal menghapus user.")
    }
  }

  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Users</h1>
        <p className="text-sm text-muted-foreground">
          Kelola akun pengguna organisasi.
        </p>
      </header>

      <form
        onSubmit={submit}
        className="space-y-4 rounded-xl border border-border bg-card p-6"
      >
        <div className="grid gap-4 sm:grid-cols-3">
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              value={form.username}
              onChange={(e) => setForm({ ...form, username: e.target.value })}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="name">Nama</Label>
            <Input
              id="name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
            />
          </div>
        </div>
        {formError ? (
          <p className="text-sm text-destructive">{formError}</p>
        ) : null}
        <Button type="submit" disabled={saving}>
          <Plus className="size-4" aria-hidden />
          Tambah user
        </Button>
      </form>

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
          <table className="w-full text-sm">
            <thead className="border-b border-border text-left text-xs uppercase tracking-wider text-muted-foreground">
              <tr>
                <th className="px-6 py-3 font-medium">Username</th>
                <th className="px-6 py-3 font-medium">Nama</th>
                <th className="px-6 py-3 font-medium">Email</th>
                <th className="px-6 py-3 font-medium">Status</th>
                <th className="px-6 py-3 text-right font-medium">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {data.map((u) => (
                <tr key={u.pk} className="border-b border-border last:border-0">
                  <td className="px-6 py-3 font-medium">{u.username}</td>
                  <td className="px-6 py-3">{u.name}</td>
                  <td className="px-6 py-3 text-muted-foreground">
                    {u.email || "—"}
                  </td>
                  <td className="px-6 py-3">
                    <span
                      className={
                        u.is_active ? "text-foreground" : "text-muted-foreground"
                      }
                    >
                      {u.is_active ? "Aktif" : "Nonaktif"}
                    </span>
                  </td>
                  <td className="px-6 py-3 text-right">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => remove(u.pk)}
                      aria-label={`Hapus ${u.username}`}
                    >
                      <Trash2 className="size-4" aria-hidden />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
