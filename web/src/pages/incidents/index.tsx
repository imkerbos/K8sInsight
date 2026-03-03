import { Table, Tag, Space, Input, Select, Button } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import dayjs from '../../utils/dayjs'
import { listIncidents } from '../../api/incidents'
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
  const [namespace, setNamespace] = useState('')
  const [state, setState] = useState('')
  const [anomalyType, setAnomalyType] = useState('')
  const [ownerName, setOwnerName] = useState('')
  const [cursorStack, setCursorStack] = useState<Array<{ lastSeen: string; id: string } | null>>([null])

  const currentCursor = cursorStack[page - 1]
  const canPrev = page > 1

  const { data, isLoading } = useQuery({
    queryKey: ['incidents', { page, namespace, state, anomalyType, ownerName, currentCursor }],
    queryFn: () => listIncidents({
      page,
      pageSize: 20,
      namespace,
      state,
      type: anomalyType,
      ownerName,
      cursorLastSeen: currentCursor?.lastSeen,
      cursorId: currentCursor?.id,
      includeTotal: page === 1,
    }),
  })

  const hasMore = !!data?.hasMore

  const resetPaging = () => {
    setPage(1)
    setCursorStack([null])
  }

  const goNext = () => {
    if (!hasMore || !data?.nextCursorLastSeen || !data?.nextCursorId) return
    const nextCursor = { lastSeen: data.nextCursorLastSeen, id: data.nextCursorId }
    setCursorStack(prev => {
      if (prev.length > page) return prev
      return [...prev, nextCursor]
    })
    setPage(prev => prev + 1)
  }

  const goPrev = () => {
    if (!canPrev) return
    setPage(prev => prev - 1)
  }

  const columns: ColumnsType<Incident> = [
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
        pagination={false}
        onRow={(record) => ({
          onClick: () => navigate(`/incidents/${record.id}`),
          style: { cursor: 'pointer' },
        })}
      />
      <Space style={{ marginTop: 16 }}>
        <Button onClick={goPrev} disabled={!canPrev}>上一页</Button>
        <Button onClick={goNext} disabled={!hasMore}>下一页</Button>
        <span>第 {page} 页</span>
        {typeof data?.total === 'number' && page === 1 && <span>（总计 {data.total} 条）</span>}
      </Space>
    </div>
  )
}
