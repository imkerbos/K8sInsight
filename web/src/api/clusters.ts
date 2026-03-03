import client from './client'
import type { Cluster } from '../types/cluster'

export async function listClusters() {
  const { data } = await client.get<{ items: Cluster[] }>('/clusters')
  return data.items ?? []
}

export async function createCluster(req: {
  name: string
  kubeconfigData: string
}) {
  const { data } = await client.post<Cluster>('/clusters', req)
  return data
}

export async function updateCluster(id: string, req: {
  name?: string
  kubeconfigData?: string
}) {
  const { data } = await client.put<Cluster>(`/clusters/${id}`, req)
  return data
}

export async function deleteCluster(id: string) {
  await client.delete(`/clusters/${id}`)
}

export async function activateCluster(id: string) {
  await client.post(`/clusters/${id}/activate`)
}

export async function deactivateCluster(id: string) {
  await client.post(`/clusters/${id}/deactivate`)
}

export async function testClusterConnection(id: string) {
  const { data } = await client.post<{
    success: boolean
    error?: string
    message?: string
    version?: string
    nodeCount?: number
  }>(`/clusters/${id}/test`)
  return data
}
