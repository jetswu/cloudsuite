"use client"

import { useAdminResource } from "@/hooks/use-admin-resource"
import { adminApi, type PortalDomain } from "@/lib/api"

export function useDomains(token: string | undefined) {
  return useAdminResource<PortalDomain>(token, adminApi.listDomains)
}
