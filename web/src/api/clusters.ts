import client from './client'
import type { Cluster } from '../types/cluster'

export async function listClusters() {
  const { data } = await client.get<{ items: Cluster[] }>('/clusters')
  return data.items ?? []
}

export async function createCluster(req: {
  name: string
  kubeconfigData: string
  prometheusUrl?: string
  prometheusLabels?: string
}) {
  const { data } = await client.post<Cluster>('/clusters', req)
  return data
}

export async function updateCluster(id: string, req: {
  name?: string
  kubeconfigData?: string
  prometheusUrl?: string
  prometheusLabels?: string
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

export async function testPrometheusConnection(id: string) {
  const { data } = await client.post<{
    success: boolean
    message?: string
    error?: string
    seriesCount?: number
  }>(`/clusters/${id}/test-prometheus`)
  return data
}

export type ClusterMetrics = {
  clusterId: string
  range: string
  step: string
  series: Record<string, [number, string][]>
}

export async function getClusterMetrics(id: string, range: string = '1h') {
  const { data } = await client.get<ClusterMetrics>(`/clusters/${id}/metrics`, {
    params: { range },
    timeout: 20000,
  })
  return data
}
