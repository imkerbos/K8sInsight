import client from './client'
import type { User } from '../types/auth'

export async function listUsers() {
  const { data } = await client.get<{ items: User[] }>('/users')
  return data.items ?? []
}

export async function createUser(req: { username: string; password: string; role?: string }) {
  const { data } = await client.post('/users', req)
  return data
}

export async function toggleUserActive(id: string) {
  const { data } = await client.post(`/users/${id}/toggle-active`)
  return data
}

export async function resetUserPassword(id: string, newPassword: string) {
  await client.post(`/users/${id}/reset-password`, { newPassword })
}

export async function changeUserRole(id: string, role: string) {
  const { data } = await client.put(`/users/${id}/role`, { role })
  return data
}
