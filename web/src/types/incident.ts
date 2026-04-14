export type IncidentState = 'Detecting' | 'Active' | 'Resolved' | 'Archived'

export type AnomalyType =
  | 'CrashLoopBackOff'
  | 'OOMKilled'
  | 'ErrorExit'
  | 'RestartIncrement'
  | 'ImagePullBackOff'
  | 'CreateContainerConfigError'
  | 'FailedScheduling'
  | 'Evicted'
  | 'StateOscillation'

export interface Incident {
  id: string
  dedupKey: string
  state: IncidentState
  firstSeen: string
  lastSeen: string
  count: number
  namespace: string
  ownerKind: string
  ownerName: string
  anomalyType: AnomalyType
  message: string
  clusterId?: string
  podNames: string
  createdAt: string
  updatedAt: string
}

export interface Evidence {
  id: string
  incidentId: string
  type: string
  content: string
  error?: string
  collectedAt: string
  createdAt: string
}

export interface PaginatedResponse<T> {
  items: T[]
  total?: number
  page: number
  pageSize: number
  hasMore?: boolean
  nextCursorLastSeen?: string
  nextCursorId?: string
}
