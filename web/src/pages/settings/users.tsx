import { useState } from 'react'
import { Table, Button, Tag, Space, Modal, Form, Input, Select, message, Typography, Popconfirm } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, LockOutlined, EditOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from '../../utils/dayjs'
import { listUsers, createUser, toggleUserActive, resetUserPassword, changeUserRole } from '../../api/users'
import { listRoles } from '../../api/roles'
import type { User } from '../../types/auth'

export default function UserManagement() {
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [resetOpen, setResetOpen] = useState<string | null>(null)
  const [roleOpen, setRoleOpen] = useState<User | null>(null)
  const [createForm] = Form.useForm()
  const [resetForm] = Form.useForm()
  const [roleForm] = Form.useForm()

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: listUsers,
  })

  const { data: roles } = useQuery({
    queryKey: ['roles'],
    queryFn: listRoles,
  })

  const createMut = useMutation({
    mutationFn: createUser,
    onSuccess: () => {
      message.success('用户创建成功')
      setCreateOpen(false)
      createForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
    onError: () => message.error('创建失败，用户名可能已存在'),
  })

  const toggleMut = useMutation({
    mutationFn: toggleUserActive,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const resetMut = useMutation({
    mutationFn: ({ id, password }: { id: string; password: string }) =>
      resetUserPassword(id, password),
    onSuccess: () => {
      message.success('密码已重置')
      setResetOpen(null)
      resetForm.resetFields()
    },
    onError: () => message.error('重置失败'),
  })

  const roleMut = useMutation({
    mutationFn: ({ id, role }: { id: string; role: string }) =>
      changeUserRole(id, role),
    onSuccess: () => {
      message.success('角色已修改')
      setRoleOpen(null)
      roleForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
    onError: () => message.error('修改角色失败'),
  })

  const roleColorMap: Record<string, string> = {
    admin: 'blue',
    operator: 'orange',
    viewer: 'default',
  }

  const columns: ColumnsType<User> = [
    {
      title: '用户名',
      dataIndex: 'username',
      width: 160,
      render: (name: string) => <Typography.Text strong style={{ fontSize: 13 }}>{name}</Typography.Text>,
    },
    {
      title: '角色',
      dataIndex: 'role',
      width: 120,
      render: (role: string) => (
        <Tag color={roleColorMap[role] ?? 'default'}>{role}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'isActive',
      width: 80,
      render: (active: boolean) => (
        <Tag color={active ? 'green' : 'red'}>{active ? '启用' : '禁用'}</Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 160,
      render: (t: string) => <span style={{ fontSize: 13, color: '#8c8c8c' }}>{dayjs(t).format('YYYY-MM-DD HH:mm')}</span>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 260,
      render: (_, record) => (
        <Space size={4}>
          <Button
            size="small"
            type="text"
            icon={<EditOutlined />}
            onClick={() => {
              setRoleOpen(record)
              roleForm.setFieldsValue({ role: record.role })
            }}
          >
            角色
          </Button>
          <Popconfirm
            title={`确定${record.isActive ? '禁用' : '启用'}该用户？`}
            onConfirm={() => toggleMut.mutate(record.id)}
          >
            <Button size="small" type="text" danger={record.isActive}>
              {record.isActive ? '禁用' : '启用'}
            </Button>
          </Popconfirm>
          <Button
            size="small"
            type="text"
            icon={<LockOutlined />}
            onClick={() => setResetOpen(record.id)}
          >
            重置密码
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>用户管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          添加用户
        </Button>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={users ?? []}
        loading={isLoading}
        pagination={false}
        size="middle"
      />

      {/* 创建用户弹窗 */}
      <Modal
        title="添加用户"
        open={createOpen}
        onCancel={() => { createForm.resetFields(); setCreateOpen(false) }}
        onOk={() => createForm.submit()}
        confirmLoading={createMut.isPending}
        destroyOnHidden
      >
        <Form form={createForm} layout="vertical" onFinish={(v) => createMut.mutate(v)} style={{ marginTop: 16 }}>
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[
            { required: true, message: '请输入密码' },
            { min: 6, message: '密码��少 6 位' },
          ]}>
            <Input.Password placeholder="密码（至少 6 位）" />
          </Form.Item>
          <Form.Item name="role" label="角色" initialValue="viewer">
            <Select
              placeholder="选择角色"
              options={(roles ?? []).map((r) => ({
                label: `${r.name}${r.description ? ` - ${r.description}` : ''}`,
                value: r.name,
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 修改角色弹窗 */}
      <Modal
        title={`修改角色 - ${roleOpen?.username ?? ''}`}
        open={!!roleOpen}
        onCancel={() => { roleForm.resetFields(); setRoleOpen(null) }}
        onOk={() => roleForm.submit()}
        confirmLoading={roleMut.isPending}
        destroyOnHidden
      >
        <Form
          form={roleForm}
          layout="vertical"
          onFinish={(v) => roleOpen && roleMut.mutate({ id: roleOpen.id, role: v.role })}
          style={{ marginTop: 16 }}
        >
          <Form.Item name="role" label="角色" rules={[{ required: true, message: '请选择角色' }]}>
            <Select
              placeholder="选择角色"
              options={(roles ?? []).map((r) => ({
                label: (
                  <Space>
                    <span>{r.name}</span>
                    {r.description && <span style={{ color: '#8c8c8c', fontSize: 12 }}>{r.description}</span>}
                  </Space>
                ),
                value: r.name,
              }))}
            />
          </Form.Item>
          <div style={{ fontSize: 12, color: '#8c8c8c' }}>
            修改角色后，该用户需要重新登录以获取新权限。
          </div>
        </Form>
      </Modal>

      {/* 重置密码弹窗 */}
      <Modal
        title="重置密码"
        open={!!resetOpen}
        onCancel={() => { resetForm.resetFields(); setResetOpen(null) }}
        onOk={() => resetForm.submit()}
        confirmLoading={resetMut.isPending}
        destroyOnHidden
      >
        <Form
          form={resetForm}
          layout="vertical"
          onFinish={(v) => resetOpen && resetMut.mutate({ id: resetOpen, password: v.newPassword })}
          style={{ marginTop: 16 }}
        >
          <Form.Item name="newPassword" label="新密码" rules={[
            { required: true, message: '请输入新密码' },
            { min: 6, message: '密码至少 6 位' },
          ]}>
            <Input.Password placeholder="新密码（至少 6 位）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
