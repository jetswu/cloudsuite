"use client"

import { useState } from "react"
import { Plus, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { adminApi, type AdminGroup, type GroupRequest } from "@/lib/api"
import { useAdminResource } from "@/hooks/use-admin-resource"

const emptyForm: GroupRequest = { name: "" }

export function GroupsManager({ accessToken }: { accessToken?: string }) {
  const { data, setData, loading, error, refresh } = useAdminResource<AdminGroup>(
    accessToken,
    adminApi.listGroups,
  )
  const [form, setForm] = useState<GroupRequest>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError(null)
    try {
      const created = await adminApi.createGroup(accessToken!, form)
      setData([...data, created])
      setForm(emptyForm)
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal membuat group.")
    } finally {
      setSaving(false)
    }
  }

  async function remove(uuid: string) {
    if (!confirm("Hapus group ini?")) return
    try {
      await adminApi.deleteGroup(accessToken!, uuid)
      setData(data.filter((g) => g.pk !== uuid))
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal menghapus group.")
    }
  }

  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Groups</h1>
        <p className="text-sm text-muted-foreground">
          Kelola group dan izin anggota organisasi.
        </p>
      </header>

      <form
        onSubmit={submit}
        className="space-y-4 rounded-xl border border-border bg-card p-6"
      >
        <div className="space-y-2">
          <Label htmlFor="group-name">Nama group</Label>
          <Input
            id="group-name"
            value={form.name}
            onChange={(e) => setForm({ name: e.target.value })}
            required
          />
        </div>
        {formError ? (
          <p className="text-sm text-destructive">{formError}</p>
        ) : null}
        <Button type="submit" disabled={saving}>
          <Plus className="size-4" aria-hidden />
          Tambah group
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
                <th className="px-6 py-3 font-medium">Nama</th>
                <th className="px-6 py-3 font-medium">Superuser</th>
                <th className="px-6 py-3 font-medium">Anggota</th>
                <th className="px-6 py-3 text-right font-medium">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {data.map((g) => (
                <tr key={g.pk} className="border-b border-border last:border-0">
                  <td className="px-6 py-3 font-medium">{g.name}</td>
                  <td className="px-6 py-3">
                    {g.is_superuser ? "Ya" : "Tidak"}
                  </td>
                  <td className="px-6 py-3 text-muted-foreground">
                    {g.users.length}
                  </td>
                  <td className="px-6 py-3 text-right">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => remove(g.pk)}
                      aria-label={`Hapus ${g.name}`}
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
