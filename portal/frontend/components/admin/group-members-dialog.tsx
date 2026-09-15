"use client"

import { useEffect, useMemo, useState } from "react"
import { Loader2, Search, Trash2, UserPlus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { adminApi, type AdminUser } from "@/lib/api"

type GroupMembersDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  token: string | undefined
  group: { pk: string; name: string } | null
  onChanged: () => void
}

export function GroupMembersDialog({
  open,
  onOpenChange,
  token,
  group,
  onChanged,
}: GroupMembersDialogProps) {
  const [members, setMembers] = useState<AdminUser[]>([])
  const [allUsers, setAllUsers] = useState<AdminUser[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [query, setQuery] = useState("")
  const [addOpen, setAddOpen] = useState(false)
  const [busyPK, setBusyPK] = useState<number | null>(null)

  useEffect(() => {
    if (!open || !token || !group) return
    let cancelled = false
    setLoading(true)
    setError(null)
    setQuery("")
    setAddOpen(false)
    Promise.all([
      adminApi.listGroupMembers(token, group.pk),
      adminApi.listUsers(token),
    ])
      .then(([m, u]) => {
        if (cancelled) return
        setMembers(m)
        setAllUsers(u)
      })
      .catch((err) => {
        if (!cancelled)
          setError(err instanceof Error ? err.message : "Gagal memuat member.")
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, token, group])

  const available = useMemo(() => {
    const memberPKs = new Set(members.map((m) => m.pk))
    const q = query.trim().toLowerCase()
    return allUsers.filter(
      (u) =>
        !memberPKs.has(u.pk) &&
        (q === "" ||
          u.username.toLowerCase().includes(q) ||
          u.name.toLowerCase().includes(q) ||
          u.email.toLowerCase().includes(q)),
    )
  }, [allUsers, members, query])

  async function removeMember(pk: number) {
    if (!token || !group) return
    setBusyPK(pk)
    try {
      await adminApi.removeGroupMember(token, group.pk, pk)
      setMembers((prev) => prev.filter((m) => m.pk !== pk))
      onChanged()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menghapus member.")
    } finally {
      setBusyPK(null)
    }
  }

  async function addMember(pk: number) {
    if (!token || !group) return
    setBusyPK(pk)
    try {
      await adminApi.addGroupMember(token, group.pk, pk)
      const added = allUsers.find((u) => u.pk === pk)
      if (added) setMembers((prev) => [...prev, added])
      setQuery("")
      setAddOpen(false)
      onChanged()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menambah member.")
    } finally {
      setBusyPK(null)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Member — {group?.name}</DialogTitle>
          <DialogDescription>
            Kelola siapa saja yang tergabung dalam group ini.
          </DialogDescription>
        </DialogHeader>

        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            {members.length} member
          </p>
          <Popover open={addOpen} onOpenChange={setAddOpen}>
            <PopoverTrigger asChild>
              <Button variant="outline" size="sm">
                <UserPlus className="size-4" />
                Tambah member
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-72 p-0" align="end">
              <div className="border-b border-border p-2">
                <div className="relative">
                  <Search className="absolute top-2.5 left-2.5 size-4 text-muted-foreground" />
                  <Input
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder="Cari user…"
                    className="pl-8"
                  />
                </div>
              </div>
              <div className="max-h-56 overflow-y-auto p-1">
                {available.length === 0 ? (
                  <p className="px-3 py-2 text-sm text-muted-foreground">
                    Tidak ada user tersedia.
                  </p>
                ) : (
                  available.map((u) => (
                    <button
                      key={u.pk}
                      type="button"
                      disabled={busyPK === u.pk}
                      onClick={() => addMember(u.pk)}
                      className="flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm hover:bg-accent disabled:opacity-50"
                    >
                      <span className="flex flex-col">
                        <span className="font-medium">{u.username}</span>
                        <span className="text-xs text-muted-foreground">
                          {u.name || "—"}
                        </span>
                      </span>
                      {busyPK === u.pk ? (
                        <Loader2 className="size-4 animate-spin text-muted-foreground" />
                      ) : null}
                    </button>
                  ))
                )}
              </div>
            </PopoverContent>
          </Popover>
        </div>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        <div className="max-h-72 overflow-y-auto rounded-lg border border-border">
          {loading ? (
            <div className="space-y-2 p-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : members.length === 0 ? (
            <p className="p-4 text-sm text-muted-foreground">
              Belum ada member.
            </p>
          ) : (
            members.map((m) => (
              <div
                key={m.pk}
                className="flex items-center justify-between border-b border-border px-4 py-2.5 last:border-0"
              >
                <div className="flex min-w-0 flex-col">
                  <span className="truncate text-sm font-medium">
                    {m.username}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">
                    {m.name || m.email || "—"}
                  </span>
                </div>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  disabled={busyPK === m.pk}
                  onClick={() => removeMember(m.pk)}
                  aria-label={`Hapus ${m.username}`}
                >
                  {busyPK === m.pk ? (
                    <Loader2 className="size-4 animate-spin" />
                  ) : (
                    <Trash2 className="size-4" />
                  )}
                </Button>
              </div>
            ))
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Tutup
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
