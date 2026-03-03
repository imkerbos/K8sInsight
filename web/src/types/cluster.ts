export interface Cluster {
  id: string
  name: string
  status: 'active' | 'inactive'
  connectionStatus: 'unknown' | 'connected' | 'failed'
  statusMessage?: string
  version?: string
  nodeCount: number
  createdAt: string
  updatedAt: string
}
