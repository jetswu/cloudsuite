"use client"

// Sprint 1.3b: shared DKIM mode selector (RSA / Ed25519 / Dual) with
// per-option tooltips, used in the create-domain dialog and the domain
// setup page's "Ubah Mode DKIM" dialog.

import { Info, TriangleAlert } from "lucide-react"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import type { DKIMMode } from "@/lib/api"
import { cn } from "cn"

const MODE_OPTIONS: Array<{
  value: DKIMMode
  label: string
  desc: string
  tooltip: string
  isDefault?: boolean
}> = [
  {
    value: "rsa",
    label: "RSA saja",
    desc: "Universal — semua provider support.",
    tooltip:
      "Cocok untuk semua provider email. Lebih lambat verifikasi, tapi universal. Rekomendasi untuk pemula.",
    isDefault: true,
  },
  {
    value: "ed25519",
    label: "Ed25519 saja",
    desc: "Cepat & ringan.",
    tooltip:
      "Lebih cepat & ringan, tapi provider email lama mungkin reject → email masuk spam. Hati-hati.",
  },
  {
    value: "dual",
    label: "Dual (RSA + Ed25519)",
    desc: "Best practice. Butuh 2 DNS record.",
    tooltip:
      "Best practice industri. Email di-sign 2 cara — penerima modern pakai Ed25519 (cepat), penerima lama pakai RSA (universal). Butuh 2 DNS record.",
  },
]

export function DKIMModeSelector({
  value,
  onChange,
  disabled = false,
  showEd25519Warning = false,
  className,
}: {
  value: DKIMMode
  onChange: (mode: DKIMMode) => void
  disabled?: boolean
  showEd25519Warning?: boolean
  className?: string
}) {
  return (
    <div className={cn("space-y-3", className)}>
      <RadioGroup
        value={value}
        onValueChange={(v) => onChange(v as DKIMMode)}
        disabled={disabled}
        aria-label="Mode DKIM"
      >
        {MODE_OPTIONS.map((opt) => (
          <div key={opt.value} className="flex items-start gap-3">
            <RadioGroupItem
              value={opt.value}
              id={`dkim-mode-${opt.value}`}
              className="mt-0.5"
            />
            <div className="space-y-0.5">
              <div className="flex items-center gap-2">
                <Label
                  htmlFor={`dkim-mode-${opt.value}`}
                  className="font-medium"
                >
                  {opt.label}
                </Label>
                {opt.isDefault ? (
                  <span className="rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-secondary-foreground">
                    Default
                  </span>
                ) : null}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      aria-label={`Info mode ${opt.label}`}
                      className="text-muted-foreground transition-colors hover:text-foreground"
                    >
                      <Info className="size-3.5" aria-hidden />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent className="max-w-64 text-left">
                    {opt.tooltip}
                  </TooltipContent>
                </Tooltip>
              </div>
              <p className="text-sm text-muted-foreground">{opt.desc}</p>
            </div>
          </div>
        ))}
      </RadioGroup>
      {showEd25519Warning && value === "ed25519" ? (
        <div className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>
            Provider email lama mungkin reject email Anda. Pertimbangkan Dual
            untuk keamanan maksimal.
          </p>
        </div>
      ) : null}
    </div>
  )
}
