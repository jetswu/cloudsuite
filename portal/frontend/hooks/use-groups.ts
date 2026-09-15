"use client"

import { useAdminResource } from "@/hooks/use-admin-resource"
import { adminApi, type AdminGroup } from "@/lib/api"

export function useGroups(token: string | undefined) {
  return useAdminResource<AdminGroup>(token, adminApi.listGroups)
}
