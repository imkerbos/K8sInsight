import client from './client'
import type { Role, PermissionItem } from '../types/auth'

export async function listRoles(): Promise<Role[]> {
  const { data } = await client.get<{ items: Role[] }>('/roles')
  return data.items
}

export async function createRole(req: { name: string; description: string; permissions: string[] }): Promise<Role> {
  const { data } = await client.post<Role>('/roles', req)
  return data
}

export async function updateRole(id: string, req: { name: string; description: string; permissions: string[] }): Promise<Role> {
  const { data } = await client.put<Role>(`/roles/${id}`, req)
  return data
}

export async function deleteRole(id: string): Promise<void> {
  await client.delete(`/roles/${id}`)
}

export async function listPermissions(): Promise<PermissionItem[]> {
  const { data } = await client.get<{ items: PermissionItem[] }>('/permissions')
  return data.items
}
