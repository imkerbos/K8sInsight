import { useState } from 'react'
import { Table, Button, Tag, Modal, Form, Input, Select, message, Popconfirm, Typography, Switch } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from '../../utils/dayjs'
import {
  listMonitorRules,
  createMonitorRule,
  deleteMonitorRule,
  toggleMonitorRule,
} from '../../api/monitorRules'
import { listClusters } from '../../api/clusters'
import { useAuth } from '../../contexts/AuthContext'
import { hasPermission } from '../../utils/permission'
import type { MonitorRule } from '../../types/monitorRule'

const { TextArea } = Input

const anomalyTypeOptions = [
  { label: 'CrashLoopBackOff', value: 'CrashLoopBackOff' },
  { label: 'OOMKilled', value: 'OOMKilled' },
  { label: 'ErrorExit', value: 'ErrorExit' },
  { label: 'RestartIncrement', value: 'RestartIncrement' },
  { label: 'ImagePullBackOff', value: 'ImagePullBackOff' },
  { label: 'FailedScheduling', value: 'FailedScheduling' },
  { label: 'Evicted', value: 'Evicted' },
  { label: 'StateOscillation', value: 'StateOscillation' },
]

export default function MonitorRuleList() {
  const queryClient = useQueryClient()
  const { permissions } = useAuth()
  const canWrite = hasPermission(permissions, 'rule:write')
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()

  const { data: rules, isLoading } = useQuery({
    queryKey: ['monitor-rules'],
    queryFn: listMonitorRules,
  })

  const { data: clusters } = useQuery({
    queryKey: ['clusters'],
    queryFn: listClusters,
  })

  // 已有规则的集群 ID 集合
  const usedClusterIds = new Set((rules ?? []).map(r => r.clusterId))

  // 可选集群：排除已有规则的
  const availableClusters = (clusters ?? []).filter(c => !usedClusterIds.has(c.id))

  const createMut = useMutation({
    mutationFn: (values: { clusterId: string; name: string; description?: string; watchScope?: string; watchNamespaces?: string[]; labelSelector?: string; anomalyTypes?: string[] }) => {
      const { anomalyTypes, watchNamespaces, ...rest } = values
      return createMonitorRule({
        ...rest,
        watchNamespaces: watchNamespaces?.length ? watchNamespaces.join(',') : undefined,
        anomalyTypes: anomalyTypes?.length ? JSON.stringify(anomalyTypes) : undefined,
      })
    },
    onSuccess: () => {
      message.success('监控规则创建成功')
      setModalOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['monitor-rules'] })
    },
    onError: () => message.error('创建失败'),
  })

  const deleteMut = useMutation({
    mutationFn: deleteMonitorRule,
    onSuccess: () => {
      message.success('监控规则已删除')
      queryClient.invalidateQueries({ queryKey: ['monitor-rules'] })
    },
  })

  const toggleMut = useMutation({
    mutationFn: toggleMonitorRule,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['monitor-rules'] })
    },
  })

  // 集群名称映射
  const clusterNameMap = new Map((clusters ?? []).map(c => [c.id, c.name]))

  const columns: ColumnsType<MonitorRule> = [
    {
      title: '规则名称',
      dataIndex: 'name',
      width: 160,
      render: (name: string) => <Typography.Text strong style={{ fontSize: 13 }}>{name}</Typography.Text>,
    },
    {
      title: '关联集群',
      dataIndex: 'clusterId',
      width: 140,
      render: (id: string) => <span style={{ fontSize: 13 }}>{clusterNameMap.get(id) ?? id}</span>,
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 70,
      render: (enabled: boolean, record) => (
        <Switch checked={enabled} onChange={() => toggleMut.mutate(record.id)} size="small" disabled={!canWrite} />
      ),
    },
    {
      title: '监控范围',
      dataIndex: 'watchScope',
      width: 160,
      render: (s: string, record) => {
        if (s !== 'namespaces' || !record.watchNamespaces) {
          return <span style={{ fontSize: 13 }}>全集群</span>
        }
        const nsList = record.watchNamespaces.split(',').filter(Boolean)
        return (
          <span style={{ fontSize: 13 }}>
            {nsList.map(ns => <Tag key={ns} style={{ fontSize: 12 }}>{ns}</Tag>)}
          </span>
        )
      },
    },
    {
      title: '标签选择器',
      dataIndex: 'labelSelector',
      ellipsis: true,
      render: (s: string) => <span style={{ fontSize: 13, color: s ? '#333' : '#bbb' }}>{s || '-'}</span>,
    },
    {
      title: '异常类型',
      dataIndex: 'anomalyTypes',
      width: 200,
      ellipsis: true,
      render: (s: string) => {
        if (!s) return <Tag>全部</Tag>
        try {
          const types = JSON.parse(s) as string[]
          return types.map(t => <Tag key={t} style={{ fontSize: 12 }}>{t}</Tag>)
        } catch {
          return s
        }
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
            width: 80,
            render: (_: unknown, record: MonitorRule) => (
              <Popconfirm title="确定删除该规则？" onConfirm={() => deleteMut.mutate(record.id)}>
                <Button size="small" type="text" danger icon={<DeleteOutlined />}>删除</Button>
              </Popconfirm>
            ),
          } as const,
        ]
      : []),
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>监控规则</h2>
        {canWrite && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
            添加规则
          </Button>
        )}
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={rules ?? []}
        loading={isLoading}
        pagination={false}
        size="middle"
      />

      <Modal
        title="添加监控规则"
        open={modalOpen}
        onCancel={() => { form.resetFields(); setModalOpen(false) }}
        onOk={() => form.submit()}
        confirmLoading={createMut.isPending}
        width={600}
        destroyOnHidden
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => createMut.mutate(values)}
          style={{ marginTop: 16 }}
        >
          <Form.Item name="clusterId" label="关联集群" rules={[{ required: true, message: '请选择集群' }]}>
            <Select
              placeholder="选择集群"
              options={availableClusters.map(c => ({ label: c.name, value: c.id }))}
              notFoundContent="无可用集群（所有集群已配置规则）"
            />
          </Form.Item>
          <Form.Item name="name" label="规则名称" rules={[{ required: true, message: '请输入规则名称' }]}>
            <Input placeholder="例如: 生产集群监控规则" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="可选描述" />
          </Form.Item>
          <Form.Item name="watchScope" label="监控范围" initialValue="cluster">
            <Select options={[
              { label: '全集群', value: 'cluster' },
              { label: '指定命名空间', value: 'namespaces' },
            ]} />
          </Form.Item>
          <Form.Item noStyle dependencies={['watchScope']}>
            {({ getFieldValue }) =>
              getFieldValue('watchScope') === 'namespaces' ? (
                <Form.Item
                  name="watchNamespaces"
                  label="命名空间"
                  rules={[{ required: true, message: '请输入至少一个命名空间' }]}
                >
                  <Select mode="tags" placeholder="输入命名空间名称后回车" tokenSeparators={[',']} />
                </Form.Item>
              ) : null
            }
          </Form.Item>
          <Form.Item name="labelSelector" label="标签选择器">
            <Input placeholder="可选，例如: app=nginx" />
          </Form.Item>
          <Form.Item name="anomalyTypes" label="异常类型（留空=全部检测）">
            <Select mode="multiple" placeholder="选择要检测的异常类型" options={anomalyTypeOptions} allowClear />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
