import { useState } from 'react'
import { Table, Button, Tag, Space, Modal, Form, Input, message, Typography, Popconfirm, Checkbox } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from '../../utils/dayjs'
import { listRoles, createRole, updateRole, deleteRole, listPermissions } from '../../api/roles'
import type { Role } from '../../types/auth'

export default function RoleManagement() {
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editingRole, setEditingRole] = useState<Role | null>(null)
  const [form] = Form.useForm()

  const { data: roles, isLoading } = useQuery({
    queryKey: ['roles'],
    queryFn: listRoles,
  })

  const { data: permissions } = useQuery({
    queryKey: ['permissions'],
    queryFn: listPermissions,
  })

  const createMut = useMutation({
    mutationFn: createRole,
    onSuccess: () => {
      message.success('角色创建成功')
      closeModal()
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
    onError: () => message.error('创建失败，角色名可能已存在'),
  })

  const updateMut = useMutation({
    mutationFn: ({ id, ...req }: { id: string; name: string; description: string; permissions: string[] }) =>
      updateRole(id, req),
    onSuccess: () => {
      message.success('角色更新成功')
      closeModal()
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
    onError: () => message.error('更新失败'),
  })

  const deleteMut = useMutation({
    mutationFn: deleteRole,
    onSuccess: () => {
      message.success('角色已删除')
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
    onError: () => message.error('删除失败'),
  })

  const closeModal = () => {
    setModalOpen(false)
    setEditingRole(null)
    form.resetFields()
  }

  const openCreate = () => {
    setEditingRole(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (role: Role) => {
    setEditingRole(role)
    form.setFieldsValue({
      name: role.name,
      description: role.description,
      permissions: role.permissions,
    })
    setModalOpen(true)
  }

  const onFinish = (values: { name: string; description: string; permissions: string[] }) => {
    if (editingRole) {
      updateMut.mutate({ id: editingRole.id, ...values })
    } else {
      createMut.mutate(values)
    }
  }

  const columns: ColumnsType<Role> = [
    {
      title: '角色名称',
      dataIndex: 'name',
      width: 160,
      render: (name: string, record) => (
        <Space>
          <Typography.Text strong style={{ fontSize: 13 }}>{name}</Typography.Text>
          {record.builtIn && <Tag color="blue">内置</Tag>}
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      width: 200,
      render: (desc: string) => <span style={{ color: '#595959', fontSize: 13 }}>{desc || '-'}</span>,
    },
    {
      title: '权限',
      dataIndex: 'permissions',
      render: (perms: string[]) => (
        <Space size={[4, 4]} wrap>
          {perms.map((p) => (
            <Tag key={p} style={{ margin: 0, fontSize: 12 }} color="processing">{p}</Tag>
          ))}
        </Space>
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
      width: 140,
      render: (_, record) => (
        <Space size={4}>
          <Button size="small" type="text" icon={<EditOutlined />} onClick={() => openEdit(record)}>
            编辑
          </Button>
          {!record.builtIn && (
            <Popconfirm title="确定删除该角色？" onConfirm={() => deleteMut.mutate(record.id)}>
              <Button size="small" type="text" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>角色管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          添加角色
        </Button>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={roles ?? []}
        loading={isLoading}
        pagination={false}
        size="middle"
      />

      <Modal
        title={editingRole ? '编辑角色' : '添加角色'}
        open={modalOpen}
        onCancel={closeModal}
        onOk={() => form.submit()}
        confirmLoading={createMut.isPending || updateMut.isPending}
        destroyOnHidden
        width={560}
      >
        <Form form={form} layout="vertical" onFinish={onFinish} style={{ marginTop: 16 }}>
          <Form.Item
            name="name"
            label="角色名称"
            rules={[{ required: true, message: '请输入角色名称' }]}
          >
            <Input placeholder="角色名称（英文）" disabled={editingRole?.builtIn} />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea placeholder="角色描述" rows={2} />
          </Form.Item>
          <Form.Item
            name="permissions"
            label="权限"
            rules={[{ required: true, message: '请选择至少一个权限' }]}
          >
            <Checkbox.Group style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {permissions?.map((p) => (
                <Checkbox key={p.key} value={p.key}>
                  <Space size={8}>
                    <Tag color="processing" style={{ margin: 0, fontSize: 12 }}>{p.key}</Tag>
                    <span style={{ color: '#8c8c8c', fontSize: 13 }}>{p.description}</span>
                  </Space>
                </Checkbox>
              ))}
            </Checkbox.Group>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
