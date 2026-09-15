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

export type PortalDomain = {
  id: string
  name: string
  tenant_id?: string | null
  status: DomainStatus
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
    request<void>(`/users/${id}`, token, { method: "DELETE" }),

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

  listDNSRecords: (token: string, id: string) =>
    request<DNSRecord[]>(`/domains/${id}/records`, token),
}
