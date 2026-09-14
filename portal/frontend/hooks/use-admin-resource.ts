"use client"

import { useCallback, useEffect, useState } from "react"

type Fetcher<T> = (token: string) => Promise<T[]>

export function useAdminResource<T>(
  token: string | undefined,
  fetcher: Fetcher<T>,
) {
  const [data, setData] = useState<T[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    if (!token) {
      setError("Sesi berakhir — silakan masuk ulang.")
      setLoading(false)
      return
    }
    setLoading(true)
    setError(null)
    try {
      setData(await fetcher(token))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat data.")
    } finally {
      setLoading(false)
    }
  }, [token, fetcher])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return { data, setData, loading, error, refresh }
}
