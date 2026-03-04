import { BrowserRouter, Routes, Route, Navigate, Link, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { App as AntApp, ConfigProvider, Layout, Menu, Dropdown, Avatar, Space, Modal, Form, Input, Button, message, Spin, Result, theme } from 'antd'
import {
  DashboardOutlined,
  WarningOutlined,
  UserOutlined,
  LogoutOutlined,
  KeyOutlined,
  MonitorOutlined,
  ClusterOutlined,
  EyeOutlined,
  TeamOutlined,
  SettingOutlined,
  SkinOutlined,
  InfoCircleOutlined,
  SafetyCertificateOutlined,
  BellOutlined,
  LinkOutlined,
  LockOutlined,
  FundProjectionScreenOutlined,
} from '@ant-design/icons'
import zhCN from 'antd/locale/zh_CN'
import { useState, useEffect, useMemo, type ReactNode } from 'react'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { changePassword } from './api/auth'
import { getBranding } from './api/settings'
import { hasPermission } from './utils/permission'
import Dashboard from './pages/dashboard'
import IncidentList from './pages/incidents'
import IncidentDetail from './pages/incidents/detail'
import ClusterList from './pages/clusters'
import MonitorRuleList from './pages/monitor-rules'
import UserManagement from './pages/settings/users'
import RoleManagement from './pages/settings/roles'
import SecuritySettings from './pages/settings'
import CollectSettings from './pages/settings/collect'
import NotifySettings from './pages/settings/notify'
import SSOSettings from './pages/settings/sso'
import BrandingSettings from './pages/settings/branding'
import AboutPage from './pages/settings/about'
import LoginPage from './pages/login'
import SSOCallbackPage from './pages/login/SSOCallback'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchInterval: 30000,
      retry: 1,
    },
  },
})

const { Sider, Header, Content } = Layout

// 路由守卫
function RequireAuth({ children }: { children: ReactNode }) {
  const { authenticated, loading } = useAuth()

  if (loading) {
    return (
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (!authenticated) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

// 权限守卫组件
function RequirePermission({ permission, children }: { permission: string; children: ReactNode }) {
  const { permissions } = useAuth()

  if (!hasPermission(permissions, permission)) {
    return <Result status="403" title="无权限" subTitle="您没有权限访问此���面" />
  }

  return <>{children}</>
}

// 修改密码弹窗
function ChangePasswordModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const { logout } = useAuth()

  const onFinish = async (values: { oldPassword: string; newPassword: string }) => {
    setLoading(true)
    try {
      await changePassword(values)
      message.success('密码修改成功，请重新登录')
      form.resetFields()
      onClose()
      await logout()
    } catch {
      message.error('密码修改失败，请检查原密码')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      title="修改密码"
      open={open}
      onCancel={() => { form.resetFields(); onClose() }}
      footer={null}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" onFinish={onFinish} style={{ marginTop: 16 }}>
        <Form.Item name="oldPassword" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}>
          <Input.Password placeholder="请输入当前密码" />
        </Form.Item>
        <Form.Item name="newPassword" label="新密码" rules={[
          { required: true, message: '请输入新密码' },
          { min: 6, message: '密码至少 6 位' },
        ]}>
          <Input.Password placeholder="请输入新密码（至少 6 位）" />
        </Form.Item>
        <Form.Item name="confirmPassword" label="确认新密码" dependencies={['newPassword']} rules={[
          { required: true, message: '请确认新密码' },
          ({ getFieldValue }) => ({
            validator(_, value) {
              if (!value || getFieldValue('newPassword') === value) return Promise.resolve()
              return Promise.reject(new Error('两次输入的密码不一致'))
            },
          }),
        ]}>
          <Input.Password placeholder="请再次输入新密码" />
        </Form.Item>
        <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
          <Space>
            <Button onClick={() => { form.resetFields(); onClose() }}>取消</Button>
            <Button type="primary" htmlType="submit" loading={loading}>确认修改</Button>
          </Space>
        </Form.Item>
      </Form>
    </Modal>
  )
}

// 主布局带侧边栏
function AppLayout() {
  const { user, permissions, logout } = useAuth()
  const location = useLocation()
  const [pwdModalOpen, setPwdModalOpen] = useState(false)
  const { data: branding } = useQuery({ queryKey: ['branding'], queryFn: getBranding })

  useEffect(() => {
    const title = branding?.site_title || 'K8sInsight'
    const slogan = branding?.site_slogan
    document.title = slogan ? `${title} - ${slogan}` : title
  }, [branding])

  const menuItems = useMemo(() => {
    const has = (perm: string) => hasPermission(permissions, perm)

    type MenuItem = {
      key: string
      icon: React.ReactNode
      label: React.ReactNode
      children?: MenuItem[]
    }

    const items: MenuItem[] = [
      {
        key: '/',
        icon: <DashboardOutlined />,
        label: <Link to="/">仪表盘</Link>,
      },
    ]

    if (has('incident:read')) {
      items.push({
        key: '/incidents',
        icon: <WarningOutlined />,
        label: <Link to="/incidents">异常事件</Link>,
      })
    }

    if (has('cluster:read')) {
      items.push({
        key: '/clusters',
        icon: <ClusterOutlined />,
        label: <Link to="/clusters">集群管理</Link>,
      })
    }

    if (has('rule:read')) {
      items.push({
        key: '/monitor-rules',
        icon: <EyeOutlined />,
        label: <Link to="/monitor-rules">监控规则</Link>,
      })
    }

    // 系统管理子菜单
    const settingsChildren: MenuItem[] = []

    if (has('user:manage')) {
      settingsChildren.push({
        key: '/settings/users',
        icon: <TeamOutlined />,
        label: <Link to="/settings/users">用户管理</Link>,
      })
    }

    if (has('role:manage')) {
      settingsChildren.push({
        key: '/settings/roles',
        icon: <SafetyCertificateOutlined />,
        label: <Link to="/settings/roles">角色管理</Link>,
      })
    }

    if (has('settings:manage')) {
      settingsChildren.push(
        {
          key: '/settings/branding',
          icon: <SkinOutlined />,
          label: <Link to="/settings/branding">产品设置</Link>,
        },
        {
          key: '/settings/security',
          icon: <LockOutlined />,
          label: <Link to="/settings/security">安全配置</Link>,
        },
        {
          key: '/settings/collect',
          icon: <FundProjectionScreenOutlined />,
          label: <Link to="/settings/collect">资源采集</Link>,
        },
        {
          key: '/settings/notify',
          icon: <BellOutlined />,
          label: <Link to="/settings/notify">通知配置</Link>,
        },
        {
          key: '/settings/sso',
          icon: <LinkOutlined />,
          label: <Link to="/settings/sso">SSO 认证</Link>,
        },
      )
    }

    settingsChildren.push({
      key: '/settings/about',
      icon: <InfoCircleOutlined />,
      label: <Link to="/settings/about">关于</Link>,
    })

    if (settingsChildren.length > 0) {
      items.push({
        key: 'settings',
        icon: <SettingOutlined />,
        label: '系统管理',
        children: settingsChildren,
      })
    }

    return items
  }, [permissions])

  const selectedKey = (() => {
    const p = location.pathname
    if (p.startsWith('/incidents')) return '/incidents'
    if (p.startsWith('/clusters')) return '/clusters'
    if (p.startsWith('/monitor-rules')) return '/monitor-rules'
    if (p === '/settings/users') return '/settings/users'
    if (p === '/settings/roles') return '/settings/roles'
    if (p === '/settings/branding') return '/settings/branding'
    if (p === '/settings/security') return '/settings/security'
    if (p === '/settings/collect') return '/settings/collect'
    if (p === '/settings/notify') return '/settings/notify'
    if (p === '/settings/sso') return '/settings/sso'
    if (p === '/settings/about') return '/settings/about'
    return '/'
  })()

  const userMenuItems = [
    {
      key: 'change-password',
      icon: <KeyOutlined />,
      label: '修改密码',
      onClick: () => setPwdModalOpen(true),
    },
    { type: 'divider' as const },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      danger: true,
      onClick: () => logout(),
    },
  ]

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        width={220}
        style={{
          background: 'linear-gradient(180deg, #001529 0%, #002140 100%)',
          boxShadow: '2px 0 6px rgba(0,0,0,0.08)',
        }}
      >
        <div style={{
          height: 56,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 10,
          borderBottom: '1px solid rgba(255,255,255,0.06)',
        }}>
          {branding?.site_logo ? (
            <Avatar src={branding.site_logo} shape="square" size={28} />
          ) : (
            <div style={{
              width: 28,
              height: 28,
              borderRadius: 7,
              background: 'linear-gradient(135deg, #1890ff 0%, #096dd9 100%)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}>
              <MonitorOutlined style={{ fontSize: 14, color: '#fff' }} />
            </div>
          )}
          <span style={{ color: '#fff', fontWeight: 600, fontSize: 15, letterSpacing: 0.5 }}>
            {branding?.site_title || 'K8sInsight'}
          </span>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          defaultOpenKeys={['settings']}
          items={menuItems}
          style={{ borderRight: 0, marginTop: 4, fontSize: 13 }}
        />
      </Sider>
      <Layout style={{ background: '#f0f2f5' }}>
        <Header style={{
          background: '#fff',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid #eee',
          height: 52,
          lineHeight: '52px',
        }}>
          <span style={{ fontSize: 13, color: '#999' }}>{branding?.site_slogan || 'Kubernetes 异常监测与根因分析'}</span>
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight" trigger={['click']}>
            <Space style={{ cursor: 'pointer', padding: '4px 12px', borderRadius: 6, transition: 'background 0.2s' }}>
              <Avatar size={28} icon={<UserOutlined />} style={{ background: '#1890ff' }} />
              <span style={{ fontSize: 13, color: '#333', fontWeight: 500 }}>{user?.username}</span>
            </Space>
          </Dropdown>
        </Header>
        <Content style={{ margin: 16, overflow: 'auto' }}>
          <div style={{
            padding: 20,
            background: '#fff',
            borderRadius: 8,
            border: '1px solid #f0f0f0',
            minHeight: 'calc(100vh - 116px)',
          }}>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/incidents" element={
                <RequirePermission permission="incident:read"><IncidentList /></RequirePermission>
              } />
              <Route path="/incidents/:id" element={
                <RequirePermission permission="incident:read"><IncidentDetail /></RequirePermission>
              } />
              <Route path="/clusters" element={
                <RequirePermission permission="cluster:read"><ClusterList /></RequirePermission>
              } />
              <Route path="/monitor-rules" element={
                <RequirePermission permission="rule:read"><MonitorRuleList /></RequirePermission>
              } />
              <Route path="/settings/users" element={
                <RequirePermission permission="user:manage"><UserManagement /></RequirePermission>
              } />
              <Route path="/settings/roles" element={
                <RequirePermission permission="role:manage"><RoleManagement /></RequirePermission>
              } />
              <Route path="/settings/branding" element={
                <RequirePermission permission="settings:manage"><BrandingSettings /></RequirePermission>
              } />
              <Route path="/settings/security" element={
                <RequirePermission permission="settings:manage"><SecuritySettings /></RequirePermission>
              } />
              <Route path="/settings/collect" element={
                <RequirePermission permission="settings:manage"><CollectSettings /></RequirePermission>
              } />
              <Route path="/settings/notify" element={
                <RequirePermission permission="settings:manage"><NotifySettings /></RequirePermission>
              } />
              <Route path="/settings/sso" element={
                <RequirePermission permission="settings:manage"><SSOSettings /></RequirePermission>
              } />
              <Route path="/settings/about" element={<AboutPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </div>
        </Content>
      </Layout>
      <ChangePasswordModal open={pwdModalOpen} onClose={() => setPwdModalOpen(false)} />
    </Layout>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider
        locale={zhCN}
        theme={{
          algorithm: theme.defaultAlgorithm,
          token: {
            borderRadius: 6,
            colorPrimary: '#1890ff',
          },
          components: {
            Card: {
              boxShadow: 'none',
              boxShadowTertiary: 'none',
            },
            Table: {
              headerBg: '#fafafa',
              borderColor: '#f0f0f0',
            },
          },
        }}
      >
        <AntApp>
          <AuthProvider>
            <BrowserRouter>
              <Routes>
                <Route path="/login" element={<LoginRoute />} />
                <Route path="/auth/sso/callback" element={<SSOCallbackPage />} />
                <Route path="/*" element={
                  <RequireAuth>
                    <AppLayout />
                  </RequireAuth>
                } />
              </Routes>
            </BrowserRouter>
          </AuthProvider>
        </AntApp>
      </ConfigProvider>
    </QueryClientProvider>
  )
}

// 已登录时访问 /login 自动跳转首页
function LoginRoute() {
  const { authenticated, loading } = useAuth()

  if (loading) {
    return (
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (authenticated) {
    return <Navigate to="/" replace />
  }

  return <LoginPage />
}
