import { Card, Col, Row, Statistic, Table, Tag, Empty } from 'antd'
import {
  WarningOutlined,
  FireOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  ClusterOutlined,
} from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
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

export default function Dashboard() {
  const navigate = useNavigate()

  const { data: activeData } = useQuery({
    queryKey: ['incidents', 'active'],
    queryFn: () => listIncidents({ state: 'Active', pageSize: 1, includeTotal: true }),
  })
  const { data: detectingData } = useQuery({
    queryKey: ['incidents', 'detecting'],
    queryFn: () => listIncidents({ state: 'Detecting', pageSize: 1, includeTotal: true }),
  })
  const { data: resolvedData } = useQuery({
    queryKey: ['incidents', 'resolved'],
    queryFn: () => listIncidents({ state: 'Resolved', pageSize: 1, includeTotal: true }),
  })
  const { data: recentData } = useQuery({
    queryKey: ['incidents', 'recent'],
    queryFn: () => listIncidents({ pageSize: 5 }),
  })
  const { data: clusters } = useQuery({
    queryKey: ['clusters'],
    queryFn: listClusters,
  })

  const activeClusters = clusters?.filter((c) => c.status === 'active').length ?? 0
  const totalClusters = clusters?.length ?? 0

  const statCards = [
    { title: '活跃异常', value: activeData?.total ?? 0, icon: <FireOutlined />, color: '#cf1322', bg: '#fff1f0', border: '#ffccc7' },
    { title: '检测中', value: detectingData?.total ?? 0, icon: <ClockCircleOutlined />, color: '#fa8c16', bg: '#fff7e6', border: '#ffd591' },
    { title: '已解决', value: resolvedData?.total ?? 0, icon: <CheckCircleOutlined />, color: '#3f8600', bg: '#f6ffed', border: '#b7eb8f' },
    { title: '事件总数', value: (activeData?.total ?? 0) + (detectingData?.total ?? 0) + (resolvedData?.total ?? 0), icon: <WarningOutlined />, color: '#595959', bg: '#fafafa', border: '#f0f0f0' },
  ]

  return (
    <div>
      <div style={{ marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>集群概览</h2>
      </div>

      <div style={{ display: 'flex', gap: 12, marginBottom: 20 }}>
        {statCards.map((item) => (
          <div key={item.title} style={{
            flex: 1,
            padding: '16px 20px',
            background: item.bg,
            borderRadius: 8,
            border: `1px solid ${item.border}`,
          }}>
            <Statistic
              title={<span style={{ fontSize: 12, color: '#8c8c8c' }}>{item.title}</span>}
              value={item.value}
              prefix={<span style={{ fontSize: 16 }}>{item.icon}</span>}
              styles={{ content: { color: item.color, fontSize: 24, fontWeight: 600 } }}
            />
          </div>
        ))}
      </div>

      <Row gutter={16}>
        <Col span={16}>
          <Card
            title={<span style={{ fontSize: 14, fontWeight: 500 }}>最近事件</span>}
            size="small"
            style={{ border: '1px solid #f0f0f0', borderRadius: 8 }}
            styles={{ header: { borderBottom: '1px solid #f5f5f5' } }}
          >
            {(recentData?.items?.length ?? 0) > 0 ? (
              <Table<Incident>
                rowKey="id"
                dataSource={recentData?.items ?? []}
                pagination={false}
                size="small"
                onRow={(record) => ({
                  onClick: () => navigate(`/incidents/${record.id}`),
                  style: { cursor: 'pointer' },
                })}
                columns={[
                  {
                    title: '状态',
                    dataIndex: 'state',
                    width: 80,
                    render: (s: IncidentState) => <Tag color={stateColors[s]}>{s}</Tag>,
                  },
                  {
                    title: '类型',
                    dataIndex: 'anomalyType',
                    width: 140,
                    render: (t: AnomalyType) => <Tag color={typeColors[t]}>{t}</Tag>,
                  },
                  {
                    title: '命名空间',
                    dataIndex: 'namespace',
                    width: 120,
                  },
                  {
                    title: 'Owner',
                    key: 'owner',
                    render: (_, r) => `${r.ownerKind}/${r.ownerName}`,
                    ellipsis: true,
                  },
                  {
                    title: '时间',
                    dataIndex: 'lastSeen',
                    width: 140,
                    render: (t: string) => dayjs(t).format('MM-DD HH:mm'),
                  },
                ]}
              />
            ) : (
              <Empty description="暂无事件" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        </Col>
        <Col span={8}>
          <Card
            title={<span style={{ fontSize: 14, fontWeight: 500 }}>集群状态</span>}
            extra={<span style={{ fontSize: 13, color: '#1890ff' }}><ClusterOutlined /> {activeClusters} / {totalClusters}</span>}
            size="small"
            style={{ border: '1px solid #f0f0f0', borderRadius: 8 }}
            styles={{ header: { borderBottom: '1px solid #f5f5f5' } }}
          >
            {totalClusters > 0 ? (
              clusters?.map((c) => (
                <div key={c.id} style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '10px 0',
                  borderBottom: '1px solid #fafafa',
                }}>
                  <span style={{ fontWeight: 500, fontSize: 13 }}>{c.name}</span>
                  <Tag color={c.status === 'active' ? (c.connectionStatus === 'failed' ? 'red' : 'green') : 'default'}>
                    {c.status === 'active' ? (c.connectionStatus === 'failed' ? '连接失败' : '运行中') : '未激活'}
                  </Tag>
                </div>
              ))
            ) : (
              <Empty description="暂无集群" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}
