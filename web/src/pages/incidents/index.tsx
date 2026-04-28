import { Table, Tag, Space, Input, Select } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import dayjs from '../../utils/dayjs'
import { listIncidents } from '../../api/incidents'
import { listClusters } from '../../api/clusters'
import type { Incident, IncidentState, AnomalyType } from '../../types/incident'

const stateColors: Record<IncidentState, string> = {
  Detecting: 'orange',
  Active: 'red',
  Resolved: 'green',
  Archived: 'default',
}

const typeColors: Record<AnomalyType, string> = {
  CrashLoopBackOff: 'volcano',
  OOMKilled: 'red',
  ErrorExit: 'magenta',
  RestartIncrement: 'orange',
  ImagePullBackOff: 'gold',
  CreateContainerConfigError: 'geekblue',
  FailedScheduling: 'purple',
  Evicted: 'cyan',
  StateOscillation: 'blue',
}

export default function IncidentList() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [clusterId, setClusterId] = useState('')
  const [namespace, setNamespace] = useState('')
  const [state, setState] = useState('')
  const [anomalyType, setAnomalyType] = useState('')
  const [ownerName, setOwnerName] = useState('')

  const { data: clusters } = useQuery({
    queryKey: ['clusters'],
    queryFn: listClusters,
  })
  const clusterNameMap = useMemo(() => new Map((clusters ?? []).map(c => [c.id, c.name])), [clusters])

  const { data, isLoading } = useQuery({
    queryKey: ['incidents', { page, pageSize, clusterId, namespace, state, anomalyType, ownerName }],
    queryFn: () => listIncidents({
      page,
      pageSize,
      clusterId: clusterId || undefined,
      namespace,
      state,
      type: anomalyType,
      ownerName,
      includeTotal: true,
    }),
  })

  const resetPaging = () => {
    setPage(1)
  }

  const columns: ColumnsType<Incident> = [
    {
      title: '集群',
      dataIndex: 'clusterId',
      width: 120,
      render: (id: string) => clusterNameMap.get(id) ?? id ?? '-',
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 100,
      render: (s: IncidentState) => <Tag color={stateColors[s]}>{s}</Tag>,
    },
    {
      title: '异常类型',
      dataIndex: 'anomalyType',
      width: 160,
      render: (t: AnomalyType) => <Tag color={typeColors[t]}>{t}</Tag>,
    },
    {
      title: '命名空间',
      dataIndex: 'namespace',
      width: 140,
    },
    {
      title: 'Owner',
      key: 'owner',
      width: 200,
      render: (_, r) => r.ownerKind ? `${r.ownerKind}/${r.ownerName}` : r.ownerName,
    },
    {
      title: '消息',
      dataIndex: 'message',
      ellipsis: true,
    },
    {
      title: '次数',
      dataIndex: 'count',
      width: 80,
      align: 'center',
    },
    {
      title: '最后发现',
      dataIndex: 'lastSeen',
      width: 180,
      render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss'),
    },
  ]

  return (
    <div>
      <h2>异常事件</h2>
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          placeholder="全部集群"
          allowClear
          style={{ width: 160 }}
          onChange={v => { setClusterId(v ?? ''); resetPaging() }}
          options={(clusters ?? []).map(c => ({ label: c.name, value: c.id }))}
        />
        <Input
          placeholder="命名空间"
          allowClear
          style={{ width: 160 }}
          onChange={e => { setNamespace(e.target.value); resetPaging() }}
        />
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 140 }}
          onChange={v => { setState(v ?? ''); resetPaging() }}
          options={[
            { label: 'Active', value: 'Active' },
            { label: 'Detecting', value: 'Detecting' },
            { label: 'Resolved', value: 'Resolved' },
            { label: 'Archived', value: 'Archived' },
          ]}
        />
        <Select
          placeholder="异常类型"
          allowClear
          style={{ width: 180 }}
          onChange={v => { setAnomalyType(v ?? ''); resetPaging() }}
          options={[
            { label: 'CrashLoopBackOff', value: 'CrashLoopBackOff' },
            { label: 'OOMKilled', value: 'OOMKilled' },
            { label: 'ErrorExit', value: 'ErrorExit' },
            { label: 'RestartIncrement', value: 'RestartIncrement' },
            { label: 'ImagePullBackOff', value: 'ImagePullBackOff' },
            { label: 'CreateContainerConfigError', value: 'CreateContainerConfigError' },
            { label: 'FailedScheduling', value: 'FailedScheduling' },
            { label: 'Evicted', value: 'Evicted' },
            { label: 'StateOscillation', value: 'StateOscillation' },
          ]}
        />
        <Input.Search
          placeholder="搜索 Owner"
          allowClear
          style={{ width: 200 }}
          onSearch={v => { setOwnerName(v); resetPaging() }}
        />
      </Space>
      <Table
        rowKey="id"
        columns={columns}
        dataSource={data?.items ?? []}
        loading={isLoading}
        pagination={{
          current: page,
          pageSize,
          total: data?.total ?? 0,
          showTotal: (total) => `共 ${total} 条`,
          showSizeChanger: true,
          showQuickJumper: true,
          pageSizeOptions: ['10', '20', '50', '100'],
          onChange: (p, ps) => {
            setPage(p)
            setPageSize(ps)
          },
        }}
        onRow={(record) => ({
          onClick: () => navigate(`/incidents/${record.id}`),
          style: { cursor: 'pointer' },
        })}
      />
    </div>
  )
}
