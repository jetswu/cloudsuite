"use client"

import { useEffect, useState } from "react"
import { Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { adminApi, type AdminGroup, type AdminUser } from "@/lib/api"

type UserFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  token: string | undefined
  user: AdminUser | null
  groups: AdminGroup[]
  onSaved: (user: AdminUser) => void
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

export function UserFormDialog({
  open,
  onOpenChange,
  token,
  user,
  groups,
  onSaved,
}: UserFormDialogProps) {
  const isEdit = user !== null
  const [username, setUsername] = useState("")
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [isActive, setIsActive] = useState(true)
  const [selectedGroups, setSelectedGroups] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (open) {
      setUsername(user?.username ?? "")
      setName(user?.name ?? "")
      setEmail(user?.email ?? "")
      setIsActive(user?.is_active ?? true)
      setSelectedGroups(user?.groups ?? [])
      setError(null)
    }
  }, [open, user])

  function toggleGroup(uuid: string) {
    setSelectedGroups((prev) =>
      prev.includes(uuid) ? prev.filter((g) => g !== uuid) : [...prev, uuid],
    )
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!token) return
    setSaving(true)
    setError(null)
    try {
      if (isEdit) {
        const updated = await adminApi.updateUser(token, user!.pk, {
          username,
          name,
          email,
          is_active: isActive,
        })
        await adminApi.setUserGroups(token, user!.pk, selectedGroups)
        onSaved(updated)
      } else {
        const created = await adminApi.createUser(token, {
          username,
          name,
          email,
          is_active: isActive,
          groups: selectedGroups,
        })
        onSaved(created)
      }
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan user.")
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit user" : "Tambah user"}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? `Ubah data akun ${user?.username}.`
              : "Buat akun pengguna baru untuk organisasi."}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="user-username">Username</Label>
            <Input
              id="user-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="user-name">Nama</Label>
            <Input
              id="user-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="user-email">Email</Label>
            <Input
              id="user-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>

          {isEdit ? (
            <div className="flex items-center justify-between rounded-lg border border-border px-3 py-2">
              <div className="space-y-0.5">
                <p className="text-sm font-medium">Aktif</p>
                <p className="text-xs text-muted-foreground">
                  {user?.last_login
                    ? `Login terakhir ${formatDateTime(user.last_login)}`
                    : "Belum pernah login"}
                </p>
              </div>
              <label className="flex cursor-pointer items-center gap-2">
                <Checkbox
                  checked={isActive}
                  onCheckedChange={(v) => setIsActive(v === true)}
                />
                <span className="text-sm text-muted-foreground">
                  {isActive ? "Aktif" : "Ditangguhkan"}
                </span>
              </label>
            </div>
          ) : null}

          <div className="space-y-2">
            <Label>Group</Label>
            <div className="max-h-44 overflow-y-auto rounded-lg border border-border p-2">
              {groups.length === 0 ? (
                <p className="px-2 py-1 text-sm text-muted-foreground">
                  Belum ada group.
                </p>
              ) : (
                groups.map((g) => (
                  <label
                    key={g.pk}
                    className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 hover:bg-accent"
                  >
                    <Checkbox
                      checked={selectedGroups.includes(g.pk)}
                      onCheckedChange={() => toggleGroup(g.pk)}
                    />
                    <span className="text-sm">{g.name}</span>
                  </label>
                ))
              )}
            </div>
          </div>

          {error ? (
            <p className="text-sm text-destructive">{error}</p>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={saving}
            >
              Batal
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? <Loader2 className="size-4 animate-spin" /> : null}
              {isEdit ? "Simpan" : "Tambah"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
