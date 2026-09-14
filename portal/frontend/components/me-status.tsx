"use client";

import { useEffect, useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";

type Me = {
  sub?: string;
  preferred_username?: string;
};

export function MeStatus({ accessToken }: { accessToken?: string }) {
  const [state, setState] = useState<"loading" | "ok" | "error">("loading");
  const [me, setMe] = useState<Me | null>(null);

  async function load() {
    if (!accessToken) {
      setState("error");
      return;
    }
    setState("loading");
    try {
      const res = await fetch(`/api/me`, {
        headers: { Authorization: `Bearer ${accessToken}` },
        cache: "no-store",
      });
      if (!res.ok) {
        setState("error");
        return;
      }
      setMe(await res.json());
      setState("ok");
    } catch {
      setState("error");
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accessToken]);

  if (state === "loading") {
    return (
      <div className="space-y-2">
        <Skeleton className="h-4 w-48" />
        <Skeleton className="h-3 w-32" />
      </div>
    );
  }

  if (state === "error") {
    return (
      <div className="flex items-center gap-3 rounded-md border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
        <span>Gagal memuat data akun dari backend.</span>
        <button
          type="button"
          onClick={load}
          className="rounded-md px-2 py-1 text-sm font-medium text-primary underline-offset-4 transition-colors hover:underline focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          Coba lagi
        </button>
      </div>
    );
  }

  return (
    <p className="text-sm text-muted-foreground">
      Terhubung sebagai{" "}
      <span className="font-medium text-foreground">
        {me?.preferred_username ?? me?.sub ?? "—"}
      </span>
    </p>
  );
}
