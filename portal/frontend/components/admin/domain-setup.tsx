"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import {
  ArrowLeft,
  CheckCircle2,
  CheckCheck,
  Copy,
  Loader2,
  RefreshCw,
  ShieldCheck,
  Settings2,
  XCircle,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { DKIMModeSelector } from "@/components/admin/dkim-mode-selector"
import {
  adminApi,
  type DKIMMode,
  type DNSRecord,
  type DomainDetail,
} from "@/lib/api"

const STEP_ORDER = ["mx", "spf", "dkim", "dmarc"] as const

const STEP_LABEL: Record<string, string> = {
  mx: "MX",
  spf: "SPF",
  dkim: "DKIM",
  dmarc: "DMARC",
}

const STEP_DESC: Record<string, string> = {
  mx: "Arahkan email masuk ke server mail.",
  spf: "Izinkan server mail mengirim atas nama domain.",
  dkim: "Tanda tangan email agar tidak dianggap spam.",
  dmarc: "Kebijakan untuk email gagal autentikasi.",
}

const DKIM_MODE_LABEL: Record<DKIMMode, string> = {
  rsa: "RSA",
  ed25519: "Ed25519",
  dual: "Dual (RSA + Ed25519)",
}

type RecordState = "pending" | "verified" | "failed" | "mismatch"

function recordState(rec: DNSRecord | undefined): RecordState {
  if (!rec) return "pending"
  return rec.status
}

function recordTypeLabel(rec: DNSRecord): string {
  if (rec.record_type === "MX") {
    return `MX ${rec.priority ?? ""}`.trim()
  }
  return "TXT"
}

function recordHost(domainName: string, rec: DNSRecord): string {
  if (rec.name === "@") return domainName
  return `${rec.name}.${domainName}`
}

function recordValueForCopy(rec: DNSRecord): string {
  if (rec.record_type === "MX") {
    return `${recordHost("", rec)}. IN MX ${rec.priority ?? 10} ${rec.value}.`
  }
  return `"${rec.value}"`
}

export function DomainSetup({
  accessToken,
  domainId,
}: {
  accessToken?: string
  domainId: string
}) {
  const [detail, setDetail] = useState<DomainDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [verifying, setVerifying] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)
  const [modeDialogOpen, setModeDialogOpen] = useState(false)
  const [newMode, setNewMode] = useState<DKIMMode>("rsa")
  const [savingMode, setSavingMode] = useState(false)

  const load = useCallback(async () => {
    if (!accessToken) {
      setError("Sesi berakhir — silakan masuk ulang.")
      setLoading(false)
      return
    }
    setLoading(true)
    setError(null)
    try {
      setDetail(await adminApi.getDomain(accessToken, domainId))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat domain.")
    } finally {
      setLoading(false)
    }
  }, [accessToken, domainId])

  useEffect(() => {
    void load()
  }, [load])

  const recordsByPurpose = useMemo(() => {
    const map = new Map<string, DNSRecord[]>()
    for (const rec of detail?.records ?? []) {
      const arr = map.get(rec.purpose) ?? []
      arr.push(rec)
      map.set(rec.purpose, arr)
    }
    return map
  }, [detail])

  const allVerified = useMemo(() => {
    const records = detail?.records ?? []
    if (records.length === 0) return false
    const required = records.filter((r) => r.is_required)
    return required.length > 0 && required.every((r) => r.status === "verified")
  }, [detail])

  const domainName = detail?.domain.name ?? ""

  async function verifyAll() {
    if (!accessToken) return
    setVerifying(true)
    setError(null)
    try {
      setDetail(await adminApi.verifyDomain(accessToken, domainId))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal verifikasi.")
    } finally {
      setVerifying(false)
    }
  }

  async function copyValue(purpose: string, text: string) {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(purpose)
      setTimeout(() => setCopied(null), 1500)
    } catch {
      // Clipboard unavailable — ignore.
    }
  }

  async function saveMode() {
    if (!accessToken || !detail) return
    setSavingMode(true)
    setError(null)
    try {
      setDetail(await adminApi.updateDomainMode(accessToken, domainId, newMode))
      setModeDialogOpen(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengubah mode DKIM.")
    } finally {
      setSavingMode(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }

  if (error && !detail) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-destructive">{error}</p>
        <Button variant="outline" size="sm" onClick={load}>
          Coba lagi
        </Button>
      </div>
    )
  }

  if (!detail) return null

  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <Button variant="ghost" size="sm" asChild className="-ml-2">
          <Link href="/admin/domains">
            <ArrowLeft className="size-4" aria-hidden />
            Kembali ke Domains
          </Link>
        </Button>
        <h1 className="text-2xl font-semibold tracking-tight">
          Setup DNS — {domainName}
        </h1>
        <p className="text-sm text-muted-foreground">
          Publikasikan record di penyedia DNS pelanggan, lalu verifikasi.
        </p>
        <p className="text-sm text-muted-foreground">
          Mode DKIM:{" "}
          <span className="font-medium text-foreground">
            {DKIM_MODE_LABEL[detail.domain.dkim_mode] ??
              detail.domain.dkim_mode}
          </span>
        </p>
      </header>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {allVerified ? (
        <div className="flex items-start gap-3 rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-emerald-600 dark:text-emerald-400" />
          <div className="space-y-1">
            <p className="font-medium text-emerald-700 dark:text-emerald-300">
              Domain siap, lanjut provisioning
            </p>
            <p className="text-sm text-emerald-700/80 dark:text-emerald-300/80">
              Semua DNS record terverifikasi. Provisioning mailbox tersedia di
              sprint berikutnya.
            </p>
          </div>
        </div>
      ) : null}

      <ol className="space-y-4">
        {STEP_ORDER.map((purpose) => {
          const recs = recordsByPurpose.get(purpose) ?? []
          return (
            <li key={purpose}>
              <StepCard
                purpose={purpose}
                recs={recs}
                domainName={domainName}
                state={recs.every((r) => r.status === "verified") ? "verified" : recordState(recs[0])}
                copied={copied === purpose}
                onCopy={(text) => void copyValue(purpose, text)}
              />
            </li>
          )
        })}
      </ol>

      <div className="flex flex-wrap items-center gap-3">
        <Button
          variant="outline"
          onClick={() => {
            setNewMode(detail.domain.dkim_mode)
            setModeDialogOpen(true)
          }}
        >
          <Settings2 className="size-4" aria-hidden />
          Ubah Mode DKIM
        </Button>
        <Button onClick={verifyAll} disabled={verifying}>
          {verifying ? (
            <Loader2 className="size-4 animate-spin" aria-hidden />
          ) : (
            <CheckCheck className="size-4" aria-hidden />
          )}
          Verify Semua
        </Button>
        <Button variant="outline" asChild>
          <Link href="/admin/domains">
            <ArrowLeft className="size-4" aria-hidden />
            Back to Domains
          </Link>
        </Button>
      </div>

      <Dialog open={modeDialogOpen} onOpenChange={setModeDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Ubah Mode DKIM</DialogTitle>
            <DialogDescription>
              Mode baru akan me-regenerate record DNS DKIM. MX, SPF, dan DMARC
              tidak berubah; status record DKIM kembali pending hingga
              diverifikasi ulang.
            </DialogDescription>
          </DialogHeader>
          <DKIMModeSelector
            value={newMode}
            onChange={setNewMode}
            showEd25519Warning
            disabled={savingMode}
          />
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setModeDialogOpen(false)}
              disabled={savingMode}
            >
              Batal
            </Button>
            <Button
              onClick={saveMode}
              disabled={savingMode || newMode === detail.domain.dkim_mode}
            >
              {savingMode ? "Menyimpan…" : "Simpan"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function StepCard({
  purpose,
  recs,
  domainName,
  state,
  copied,
  onCopy,
}: {
  purpose: string
  recs: DNSRecord[]
  domainName: string
  state: RecordState
  copied: boolean
  onCopy: (text: string) => void
}) {
  const label = STEP_LABEL[purpose] ?? purpose
  const desc = STEP_DESC[purpose] ?? ""

  const statusIcon =
    state === "verified" ? (
      <CheckCircle2 className="size-5 text-emerald-600 dark:text-emerald-400" />
    ) : state === "failed" || state === "mismatch" ? (
      <XCircle className="size-5 text-destructive" />
    ) : (
      <RefreshCw className="size-5 text-muted-foreground" />
    )

  return (
    <div className="rounded-xl border border-border">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div className="flex items-center gap-3">
          {statusIcon}
          <div>
            <p className="text-sm font-medium">{label}</p>
            <p className="text-xs text-muted-foreground">{desc}</p>
          </div>
        </div>
        <span
          className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
            state === "verified"
              ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
              : state === "failed" || state === "mismatch"
                ? "bg-destructive/10 text-destructive"
                : "bg-muted text-muted-foreground"
          }`}
        >
          {state === "verified"
            ? "verified"
            : state === "failed"
              ? "failed"
              : state === "mismatch"
                ? "mismatch"
                : "pending"}
        </span>
      </div>

      <div className="space-y-3 px-4 py-3">
        {recs.length === 0 ? (
          <p className="text-sm text-muted-foreground">Tidak ada record.</p>
        ) : (
          recs.map((rec) => (
            <div key={rec.id} className="rounded-lg bg-muted/50 p-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 space-y-1">
                  <p className="text-xs text-muted-foreground">
                    {recordTypeLabel(rec)} · {recordHost(domainName, rec)}
                  </p>
                  <p className="break-all font-mono text-xs text-foreground">
                    {rec.value}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`Salin record ${label}`}
                  onClick={() => onCopy(recordValueForCopy(rec))}
                >
                  {copied ? (
                    <CheckCircle2 className="size-4 text-emerald-500" />
                  ) : (
                    <Copy className="size-4" />
                  )}
                </Button>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
