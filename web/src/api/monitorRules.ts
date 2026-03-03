import client from './client'
import type { MonitorRule } from '../types/monitorRule'

export async function listMonitorRules() {
  const { data } = await client.get<{ items: MonitorRule[] }>('/monitor-rules')
  return data.items ?? []
}

export async function createMonitorRule(req: {
  clusterId: string
  name: string
  description?: string
  watchScope?: string
  watchNamespaces?: string
  labelSelector?: string
  anomalyTypes?: string
}) {
  const { data } = await client.post<MonitorRule>('/monitor-rules', req)
  return data
}

export async function updateMonitorRule(id: string, req: {
  name?: string
  description?: string
  watchScope?: string
  watchNamespaces?: string
  labelSelector?: string
  anomalyTypes?: string
}) {
  const { data } = await client.put<MonitorRule>(`/monitor-rules/${id}`, req)
  return data
}

export async function deleteMonitorRule(id: string) {
  await client.delete(`/monitor-rules/${id}`)
}

export async function toggleMonitorRule(id: string) {
  const { data } = await client.post<MonitorRule>(`/monitor-rules/${id}/toggle`)
  return data
}
