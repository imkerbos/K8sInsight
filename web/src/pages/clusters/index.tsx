import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Table, Button, Tag, Space, Modal, Form, Input, message, Popconfirm, Typography, Tooltip } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  PlusOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  DeleteOutlined,
  ExclamationCircleOutlined,
  ApiOutlined,
  CheckCircleOutlined,
  EditOutlined,
  QuestionCircleOutlined,
  LoadingOutlined,
  DashboardOutlined,
} from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from '../../utils/dayjs'
import {
  listClusters,
  createCluster,
  updateCluster,
  deleteCluster,
  activateCluster,
  deactivateCluster,
  testClusterConnection,
  testPrometheusConnection,
} from '../../api/clusters'
import { useAuth } from '../../contexts/AuthContext'
import { hasPermission } from '../../utils/permission'
import type { Cluster } from '../../types/cluster'

const { TextArea } = Input

export default function ClusterList() {
  const queryClient = useQueryClient()
  const { permissions } = useAuth()
  const canWrite = hasPermission(permissions, 'cluster:write')

  // 添加/编辑弹窗
  const [modalOpen, setModalOpen] = useState(false)
  const [editingCluster, setEditingCluster] = useState<Cluster | null>(null)
  const [form] = Form.useForm()

  // 测试连接中的集群 ID
  const [testingId, setTestingId] = useState<string | null>(null)
  const [testingPromId, setTestingPromId] = useState<string | null>(null)

  const { data: clusters, isLoading } = useQuery({
    queryKey: ['clusters'],
    queryFn: listClusters,
  })

  const createMut = useMutation({
    mutationFn: createCluster,
    onSuccess: () => {
      handleModalClose()
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      message.success('集群添加成功')
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      message.error(e?.response?.data?.error || '创建失败')
    },
  })

  const updateMut = useMutation({
    mutationFn: ({ id, ...req }: { id: string; name?: string; kubeconfigData?: string; prometheusUrl?: string; prometheusLabels?: string }) =>
      updateCluster(id, req),
    onSuccess: () => {
      handleModalClose()
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      message.success('集群更新成功')
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      message.error(e?.response?.data?.error || '更新失败')
    },
  })

  const deleteMut = useMutation({
    mutationFn: deleteCluster,
    onSuccess: () => {
      message.success('集群已删除')
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      message.error(e?.response?.data?.error || '删除失败')
    },
  })

  const activateMut = useMutation({
    mutationFn: activateCluster,
    onSuccess: () => {
      message.success('集群已启用')
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      message.error(e?.response?.data?.error || '启用失败')
    },
  })

  const deactivateMut = useMutation({
    mutationFn: deactivateCluster,
    onSuccess: () => {
      message.success('集群已禁用')
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
    },
    onError: (err: unknown) => {
      const e = err as { response?: { data?: { error?: string } } }
      message.error(e?.response?.data?.error || '禁用失败')
    },
  })

  const handleTestConnection = async (id: string) => {
    setTestingId(id)
    try {
      const result = await testClusterConnection(id)
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      if (result.success) {
        message.success(`连接成功 — 版本: ${result.version}，节点: ${result.nodeCount}`)
      } else {
        message.error(`连接失败: ${result.error}`)
      }
    } catch {
      message.error('测试请求失败')
    } finally {
      setTestingId(null)
    }
  }

  const handleTestPrometheus = async (id: string) => {
    setTestingPromId(id)
    try {
      const result = await testPrometheusConnection(id)
      if (result.success) {
        message.success(result.message || '监控连接成功')
      } else {
        message.error(`监控连接失败: ${result.error}`)
      }
    } catch {
      message.error('监控测试请求失败')
    } finally {
      setTestingPromId(null)
    }
  }

  const handleEdit = (record: Cluster) => {
    setEditingCluster(record)
    form.setFieldsValue({ name: record.name, kubeconfigData: '', prometheusUrl: record.prometheusUrl || '', prometheusLabels: record.prometheusLabels || '' })
    setModalOpen(true)
  }

  const handleModalClose = () => {
    form.resetFields()
    setEditingCluster(null)
    setModalOpen(false)
  }

  const handleSubmit = (values: { name: string; kubeconfigData: string; prometheusUrl?: string; prometheusLabels?: string }) => {
    if (editingCluster) {
      const req: { id: string; name?: string; kubeconfigData?: string; prometheusUrl?: string; prometheusLabels?: string } = { id: editingCluster.id }
      if (values.name && values.name !== editingCluster.name) req.name = values.name
      if (values.kubeconfigData) req.kubeconfigData = values.kubeconfigData
      req.prometheusUrl = values.prometheusUrl ?? ''
      req.prometheusLabels = values.prometheusLabels ?? ''
      updateMut.mutate(req)
    } else {
      createMut.mutate(values)
    }
  }

  const connectionStatusMap: Record<string, { color: string; text: string; icon: React.ReactNode }> = {
    connected: { color: 'green', text: '已连接', icon: <CheckCircleOutlined /> },
    failed: { color: 'red', text: '连接失败', icon: <ExclamationCircleOutlined /> },
    unknown: { color: 'default', text: '未测试', icon: <QuestionCircleOutlined /> },
  }

  const columns: ColumnsType<Cluster> = [
    {
      title: '集群名称',
      dataIndex: 'name',
      width: 200,
      render: (name: string, record: Cluster) => (
        <Space size={8}>
          <Link to={`/clusters/${record.id}/monitor`}>
            <Typography.Text strong style={{ fontSize: 13, color: '#1890ff' }}>{name}</Typography.Text>
          </Link>
          {record.status === 'active' && record.connectionStatus === 'connected' && (
            <Link to={`/clusters/${record.id}/monitor`}>
              <DashboardOutlined style={{ fontSize: 12, color: '#8c8c8c' }} title="监控面板" />
            </Link>
          )}
        </Space>
      ),
    },
    {
      title: '启用状态',
      dataIndex: 'status',
      width: 90,
      render: (s: string) => (
        <Tag color={s === 'active' ? 'green' : 'default'}>
          {s === 'active' ? '已启用' : '已禁用'}
        </Tag>
      ),
    },
    {
      title: '连接状态',
      dataIndex: 'connectionStatus',
      width: 120,
      render: (s: string, record) => {
        const info = connectionStatusMap[s] ?? connectionStatusMap.unknown
        return (
          <Space size={4}>
            <Tag icon={info.icon} color={info.color}>{info.text}</Tag>
            {s === 'failed' && record.statusMessage && (
              <Tooltip title={record.statusMessage}>
                <ExclamationCircleOutlined style={{ color: '#ff4d4f', fontSize: 12, cursor: 'pointer' }} />
              </Tooltip>
            )}
          </Space>
        )
      },
    },
    {
      title: '版本',
      dataIndex: 'version',
      width: 120,
      render: (v: string) => <span style={{ fontSize: 13, color: v ? '#333' : '#ccc' }}>{v || '-'}</span>,
    },
    {
      title: '节点数',
      dataIndex: 'nodeCount',
      width: 70,
      render: (n: number) => <span style={{ fontSize: 13 }}>{n || '-'}</span>,
    },
    {
      title: '最新采集',
      dataIndex: 'lastEventTime',
      width: 150,
      render: (t: string | undefined) => {
        if (!t) return <span style={{ fontSize: 13, color: '#ccc' }}>暂无</span>
        const d = dayjs(t)
        const diffMin = dayjs().diff(d, 'minute')
        const color = diffMin > 10 ? '#faad14' : '#8c8c8c'
        return (
          <Tooltip title={d.format('YYYY-MM-DD HH:mm:ss')}>
            <span style={{ fontSize: 13, color }}>{d.fromNow()}</span>
          </Tooltip>
        )
      },
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 150,
      render: (t: string) => <span style={{ fontSize: 13, color: '#8c8c8c' }}>{dayjs(t).format('YYYY-MM-DD HH:mm')}</span>,
    },
    ...(canWrite
      ? [
          {
            title: '操作',
            key: 'actions',
            width: 340,
            render: (_: unknown, record: Cluster) => (
              <Space size={4}>
                <Button
                  size="small"
                  type="text"
                  icon={testingId === record.id ? <LoadingOutlined /> : <ApiOutlined />}
                  onClick={() => handleTestConnection(record.id)}
                  disabled={testingId !== null}
                  style={{ color: '#1890ff' }}
                >
                  集群
                </Button>
                <Button
                  size="small"
                  type="text"
                  icon={testingPromId === record.id ? <LoadingOutlined /> : <DashboardOutlined />}
                  onClick={() => handleTestPrometheus(record.id)}
                  disabled={testingPromId !== null || !record.prometheusUrl}
                  style={{ color: record.prometheusUrl ? '#52c41a' : '#d9d9d9' }}
                >
                  监控
                </Button>
                <Button
                  size="small"
                  type="text"
                  icon={<EditOutlined />}
                  onClick={() => handleEdit(record)}
                >
                  编辑
                </Button>
                {record.status === 'active' ? (
                  <Popconfirm title="确定禁用该集群？" onConfirm={() => deactivateMut.mutate(record.id)}>
                    <Button size="small" type="text" icon={<PauseCircleOutlined />}>
                      禁用
                    </Button>
                  </Popconfirm>
                ) : (
                  <Button
                    size="small"
                    type="text"
                    style={{ color: '#1890ff' }}
                    icon={<PlayCircleOutlined />}
                    onClick={() => activateMut.mutate(record.id)}
                  >
                    启用
                  </Button>
                )}
                <Popconfirm title="确定删除该集群？" onConfirm={() => deleteMut.mutate(record.id)}>
                  <Button size="small" type="text" danger icon={<DeleteOutlined />}>删除</Button>
                </Popconfirm>
              </Space>
            ),
          } as const,
        ]
      : []),
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>集群管理</h2>
        {canWrite && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
            添加集群
          </Button>
        )}
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={clusters ?? []}
        loading={isLoading}
        pagination={false}
        size="middle"
      />

      <Modal
        title={editingCluster ? '编辑集群' : '添加集群'}
        open={modalOpen}
        onCancel={handleModalClose}
        onOk={() => form.submit()}
        confirmLoading={createMut.isPending || updateMut.isPending}
        okText="保存"
        width={600}
        destroyOnHidden
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          style={{ marginTop: 16 }}
        >
          <Form.Item
            name="name"
            label="集群名称"
            rules={editingCluster ? [] : [{ required: true, message: '请输入集群名称' }]}
          >
            <Input placeholder="例如: production-cluster" />
          </Form.Item>
          <Form.Item
            name="kubeconfigData"
            label="Kubeconfig"
            rules={editingCluster ? [] : [{ required: true, message: '请粘贴 kubeconfig 内容' }]}
          >
            <TextArea
              rows={8}
              placeholder={editingCluster ? '留空表示不修改 kubeconfig' : '粘贴 kubeconfig YAML 内容'}
              style={{ fontFamily: 'monospace', fontSize: 12 }}
            />
          </Form.Item>
          <Form.Item
            name="prometheusUrl"
            label="Prometheus 地址"
            extra="留空则使用系统全局配置的 Prometheus 地址"
          >
            <Input placeholder="例如: http://vmselect:8481/select/0/prometheus" />
          </Form.Item>
          <Form.Item
            name="prometheusLabels"
            label="指标标签过滤"
            extra="多集群共用同一 Prometheus/VictoriaMetrics 时，填写 vmagent external_labels 来区分集群数据"
          >
            <Input placeholder={'例如: cluster="biz-1",env="prod"'} />
          </Form.Item>
          {editingCluster && (
            <div style={{ fontSize: 12, color: '#8c8c8c', marginTop: -12 }}>
              留空字段不会被修改。修改 kubeconfig 后连接状态将重置，需重新测试。
            </div>
          )}
        </Form>
      </Modal>
    </div>
  )
}
