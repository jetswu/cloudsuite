"use client"

import { useState } from "react"
import { Check, Copy, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { AdminUser } from "@/lib/api"

type UserCreatedDialogProps = {
  user: AdminUser | null
  password: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function UserCreatedDialog({
  user,
  password,
  open,
  onOpenChange,
}: UserCreatedDialogProps) {
  const [copied, setCopied] = useState(false)

  async function copyPassword() {
    if (!password) return
    try {
      await navigator.clipboard.writeText(password)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard access denied; user can select the text manually.
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>User dibuat</DialogTitle>
          <DialogDescription>
            Akun berhasil dibuat. Simpan password berikut.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid gap-1 rounded-lg border border-border p-3">
            <span className="text-xs text-muted-foreground">Username</span>
            <span className="font-mono text-sm font-medium">
              {user?.username ?? "—"}
            </span>
          </div>

          <div className="grid gap-1 rounded-lg border border-border p-3">
            <span className="text-xs text-muted-foreground">Password</span>
            <span className="font-mono text-sm font-medium break-all">
              {password ?? "—"}
            </span>
          </div>

          <div className="flex items-start gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-400">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            <p>
              Password ini ditampilkan SEKALI. Simpan dan kirim ke user via
              channel aman (WhatsApp/telepon). Tidak bisa dilihat lagi.
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Tutup
          </Button>
          <Button type="button" onClick={copyPassword} disabled={!password}>
            {copied ? (
              <Check className="size-4" />
            ) : (
              <Copy className="size-4" />
            )}
            Copy Password
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
