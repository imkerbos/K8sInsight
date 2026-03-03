export interface MonitorRule {
  id: string
  clusterId: string
  name: string
  description?: string
  enabled: boolean
  watchScope: string
  watchNamespaces?: string
  labelSelector?: string
  anomalyTypes?: string
  createdAt: string
  updatedAt: string
}
