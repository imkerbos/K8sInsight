export interface Cluster {
  id: string
  name: string
  prometheusUrl?: string
  prometheusLabels?: string
  status: 'active' | 'inactive'
  connectionStatus: 'unknown' | 'connected' | 'failed'
  statusMessage?: string
  version?: string
  nodeCount: number
  lastEventTime?: string
  createdAt: string
  updatedAt: string
}
