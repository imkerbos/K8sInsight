import client from './client'
import type { Incident, Evidence, PaginatedResponse } from '../types/incident'

export interface ListIncidentsParams {
  page?: number
  pageSize?: number
  namespace?: string
  state?: string
  type?: string
  ownerName?: string
  cursorLastSeen?: string
  cursorId?: string
  includeTotal?: boolean
}

export async function listIncidents(params: ListIncidentsParams = {}) {
  const { data } = await client.get<PaginatedResponse<Incident>>('/incidents', { params })
  return data
}

export async function getIncident(id: string) {
  const { data } = await client.get<Incident>(`/incidents/${id}`)
  return data
}

export async function getIncidentEvidences(id: string) {
  const { data } = await client.get<{ items: Evidence[] }>(`/incidents/${id}/evidences`)
  return data.items
}
