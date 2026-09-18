// Types mirror the Go backend internal/domain payloads 1:1.

export type GroupRef = {
  pk: string
  name: string
}

export type AdminUser = {
  pk: number
  username: string
  name: string
  email: string
  is_active: boolean
  is_superuser: boolean
  last_login: string | null
  date_joined: string
  groups: string[]
  groups_obj: GroupRef[]
  roles: string[]
}

// Sprint 1.4b: per-service provisioning state (user_provisioning row).
export type ProvisioningStatus =
  | "pending"
  | "provisioning"
  | "active"
  | "failed"
  | "deleted"
  | "pending_delete"

export type ProvisioningState = {
  user_id: number
  email: string
  stalwart: ProvisioningStatus
  nextcloud: ProvisioningStatus
  odoo: ProvisioningStatus
  last_error?: string
}

export type AdminUserWithProvisioning = AdminUser & {
  provisioning?: ProvisioningState | null
}

export type RetryProvisionResponse = {
  queued: string[]
  service: string
}

export type AdminGroup = {
  pk: string
  name: string
  is_superuser: boolean
  users: number[]
  users_obj: AdminUser[]
  roles: string[]
  parents: string[]
}

export type AdminRole = {
  pk: string
  name: string
}

export type UserRequest = {
  username: string
  name: string
  email?: string
  is_active?: boolean
  groups?: string[]
  roles?: string[]
}

export type CreateUserRequest = UserRequest & {
  password: string
}

export type CreateUserResponse = {
  user: AdminUser
  password: string
}

export type GroupRequest = {
  name: string
  is_superuser?: boolean
  users?: number[]
  roles?: string[]
  parents?: string[]
}

export type RoleRequest = {
  name: string
}

// --- Domain onboarding (Sprint 1.3) ---

export type DomainStatus =
  | "pending"
  | "dns_in_progress"
  | "verified"
  | "active"
  | "error"

export type DNSRecordStatus = "pending" | "verified" | "failed" | "mismatch"

export type DNSRecordPurpose = "mx" | "spf" | "dkim" | "dmarc"

// Sprint 1.3b: DKIM signing mode. rsa = Dkim1RsaSha256 (portal default),
// ed25519 = Dkim1Ed25519Sha256, dual = Automatic (Stalwart signs both).
export type DKIMMode = "rsa" | "ed25519" | "dual"

export type PortalDomain = {
  id: string
  name: string
  tenant_id?: string | null
  status: DomainStatus
  dkim_mode: DKIMMode
  stalwart_domain_id?: string | null
  verified_at?: string | null
  created_at: string
  updated_at: string
}

export type DNSRecord = {
  id: string
  domain_id: string
  record_type: "MX" | "TXT"
  name: string
  value: string
  priority?: number | null
  purpose: DNSRecordPurpose
  is_required: boolean
  status: DNSRecordStatus
  last_checked_at?: string | null
  last_error?: string | null
  created_at: string
  updated_at: string
}

export type DomainDetail = {
  domain: PortalDomain
  records: DNSRecord[]
}

export type CreateDomainRequest = {
  name: string
  dkim_mode?: DKIMMode
}

export type UpdateDomainModeRequest = {
  dkim_mode: DKIMMode
}

// --- Audit log (Sprint 1.5a) ---

export type AuditEntry = {
  id: number
  actor_id: number | null
  actor_email: string
  actor_type: string
  action: string
  target_type: string | null
  target_id: string | null
  service_code: string | null
  ip_address: string | null
  user_agent: string | null
  metadata: Record<string, unknown>
  created_at: string
}

export type AuditListResponse = {
  total: number
  page: number
  limit: number
  items: AuditEntry[]
}

export type AuditFilters = {
  actor_id?: string
  action?: string
  service?: string
  from?: string
  to?: string
  search?: string
  page?: number
  limit?: number
}

// --- Dashboard widgets (Sprint 1.5a) ---

export type MailWidgetSummary = {
  unread_count: number
  recent: {
    id: string
    from: string
    subject: string
    received_at: string
    is_read: boolean
    deep_link: string
  }[]
  cached_at: string
  provisioned: boolean
  unavailable?: boolean
  error?: string
}

export type DriveWidgetSummary = {
  storage: {
    used_bytes: number
    total_bytes: number // -3 = unlimited
    percent: number
  }
  recent: {
    name: string
    size: number
    modified_at: string
    mime: string
    deep_link: string
  }[]
  cached_at: string
  provisioned: boolean
  unavailable?: boolean
  error?: string
}

export type ErpWidgetSummary = {
  metrics: {
    key: string
    label: string
    value: number
  }[]
  cached_at: string
  unavailable?: boolean
  error?: string
}

const BASE = "/api/admin"

async function request<T>(
  path: string,
  token: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...init?.headers,
    },
  })

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `HTTP ${res.status}`)
  }

  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

export const adminApi = {
  listUsers: (token: string) => request<AdminUser[]>("/users", token),

  createUser: (token: string, body: CreateUserRequest) =>
    request<CreateUserResponse>("/users", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateUser: (token: string, id: number, body: UserRequest) =>
    request<AdminUser>(`/users/${id}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  deleteUser: (token: string, id: number) =>
    request<{ deleted: boolean; deprovision_queued: boolean }>(
      `/users/${id}`,
      token,
      { method: "DELETE" },
    ),

  retryProvisioning: (token: string, id: number, service: string) =>
    request<RetryProvisionResponse>(`/users/${id}/retry-provision`, token, {
      method: "POST",
      body: JSON.stringify({ service }),
    }),

  getProvisioning: (token: string, id: number) =>
    request<ProvisioningState>(`/users/${id}/provisioning`, token),

  setUserGroups: (token: string, id: number, groupUUIDs: string[]) =>
    request<void>(`/users/${id}/groups`, token, {
      method: "PUT",
      body: JSON.stringify({ groups: groupUUIDs }),
    }),

  listGroups: (token: string) => request<AdminGroup[]>("/groups", token),

  createGroup: (token: string, body: GroupRequest) =>
    request<AdminGroup>("/groups", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateGroup: (token: string, uuid: string, body: GroupRequest) =>
    request<AdminGroup>(`/groups/${uuid}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  deleteGroup: (token: string, uuid: string) =>
    request<void>(`/groups/${uuid}`, token, { method: "DELETE" }),

  listGroupMembers: (token: string, uuid: string) =>
    request<AdminUser[]>(`/groups/${uuid}/members`, token),

  addGroupMember: (token: string, uuid: string, userPK: number) =>
    request<void>(`/groups/${uuid}/members`, token, {
      method: "POST",
      body: JSON.stringify({ pk: userPK }),
    }),

  removeGroupMember: (token: string, uuid: string, userPK: number) =>
    request<void>(`/groups/${uuid}/members/${userPK}`, token, {
      method: "DELETE",
    }),

  listRoles: (token: string) => request<AdminRole[]>("/roles", token),

  createRole: (token: string, body: RoleRequest) =>
    request<AdminRole>("/roles", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateRole: (token: string, uuid: string, body: RoleRequest) =>
    request<AdminRole>(`/roles/${uuid}`, token, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  deleteRole: (token: string, uuid: string) =>
    request<void>(`/roles/${uuid}`, token, { method: "DELETE" }),

  // --- Domain onboarding ---

  listDomains: (token: string) => request<PortalDomain[]>("/domains", token),

  createDomain: (token: string, body: CreateDomainRequest) =>
    request<DomainDetail>("/domains", token, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  getDomain: (token: string, id: string) =>
    request<DomainDetail>(`/domains/${id}`, token),

  deleteDomain: (token: string, id: string) =>
    request<void>(`/domains/${id}`, token, { method: "DELETE" }),

  verifyDomain: (token: string, id: string) =>
    request<DomainDetail>(`/domains/${id}/verify`, token, {
      method: "POST",
    }),

  updateDomainMode: (token: string, id: string, mode: DKIMMode) =>
    request<DomainDetail>(`/domains/${id}`, token, {
      method: "PATCH",
      body: JSON.stringify({
        dkim_mode: mode,
      } satisfies UpdateDomainModeRequest),
    }),

  listDNSRecords: (token: string, id: string) =>
    request<DNSRecord[]>(`/domains/${id}/records`, token),

  // --- Audit log (Sprint 1.5a) ---

  listAuditLogs: (token: string, filters: AuditFilters = {}) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(filters)) {
      if (v !== undefined && v !== "") qs.set(k, String(v))
    }
    const q = qs.toString()
    return request<AuditListResponse>(`/audit${q ? `?${q}` : ""}`, token)
  },

  // CSV download needs the Bearer header, so it goes through fetch + blob
  // instead of a plain <a href>.
  exportAuditLogs: async (token: string, filters: AuditFilters = {}) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(filters)) {
      if (v !== undefined && v !== "" && k !== "page" && k !== "limit")
        qs.set(k, String(v))
    }
    const q = qs.toString()
    const res = await fetch(`${BASE}/audit/export${q ? `?${q}` : ""}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `audit-log-${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },
}

// Widget endpoints live under /api/widgets and degrade softly: the backend
// answers 200 with { unavailable: true, error } instead of an error status.
const WIDGET_BASE = "/api/widgets"

async function widgetRequest<T>(path: string, token: string): Promise<T> {
  const res = await fetch(`${WIDGET_BASE}${path}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  })
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `HTTP ${res.status}`)
  }
  return (await res.json()) as T
}

export const widgetApi = {
  getMailWidget: (token: string) =>
    widgetRequest<MailWidgetSummary>("/mail/summary", token),

  getDriveWidget: (token: string) =>
    widgetRequest<DriveWidgetSummary>("/drive/summary", token),

  getErpWidget: (token: string) =>
    widgetRequest<ErpWidgetSummary>("/erp/summary", token),
}
