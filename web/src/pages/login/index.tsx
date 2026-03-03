import { useState } from 'react'
import { Form, Input, Button, message, Typography, Avatar, Divider } from 'antd'
import {
  UserOutlined,
  LockOutlined,
  AlertOutlined,
  FileSearchOutlined,
  BugOutlined,
  MonitorOutlined,
  LoginOutlined,
} from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { useAuth } from '../../contexts/AuthContext'
import { getBranding } from '../../api/settings'
import { getSSOConfig, getSSOAuthorizeURL } from '../../api/auth'

const { Title, Text } = Typography

export default function LoginPage() {
  const { login } = useAuth()
  const [loading, setLoading] = useState(false)
  const [ssoLoading, setSSOLoading] = useState(false)
  const { data: branding } = useQuery({ queryKey: ['branding'], queryFn: getBranding })
  const { data: ssoConfig } = useQuery({ queryKey: ['sso-config'], queryFn: getSSOConfig })

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      await login(values.username, values.password)
    } catch {
      message.error('用户名或密码错误')
    } finally {
      setLoading(false)
    }
  }

  const onSSOLogin = async () => {
    setSSOLoading(true)
    try {
      const { authorizeUrl } = await getSSOAuthorizeURL()
      window.location.href = authorizeUrl
    } catch {
      message.error('获取 SSO 登录地址失败')
      setSSOLoading(false)
    }
  }

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      background: 'linear-gradient(135deg, #0a1628 0%, #1a2a4a 50%, #0d1f3c 100%)',
      position: 'relative',
      overflow: 'hidden',
    }}>
      {/* 背景装饰 */}
      <div style={{
        position: 'absolute',
        top: -200,
        right: -200,
        width: 600,
        height: 600,
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(24,144,255,0.08) 0%, transparent 70%)',
      }} />
      <div style={{
        position: 'absolute',
        bottom: -100,
        left: -100,
        width: 400,
        height: 400,
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(82,196,26,0.06) 0%, transparent 70%)',
      }} />

      {/* 左侧品牌区 */}
      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        paddingLeft: '10%',
        position: 'relative',
        zIndex: 1,
      }}>
        <div style={{ maxWidth: 420 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 32 }}>
            {branding?.site_logo ? (
              <Avatar src={branding.site_logo} shape="square" size={48} style={{ boxShadow: '0 8px 24px rgba(24,144,255,0.3)' }} />
            ) : (
              <div style={{
                width: 48,
                height: 48,
                borderRadius: 12,
                background: 'linear-gradient(135deg, #1890ff 0%, #096dd9 100%)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: '0 8px 24px rgba(24,144,255,0.3)',
              }}>
                <MonitorOutlined style={{ fontSize: 24, color: '#fff' }} />
              </div>
            )}
            <Title level={3} style={{ margin: 0, color: '#fff', letterSpacing: 0.5 }}>
              {branding?.site_title || 'K8sInsight'}
            </Title>
          </div>
          <Title level={2} style={{ color: 'rgba(255,255,255,0.92)', fontWeight: 500, lineHeight: 1.5, marginBottom: 16 }}>
            {branding?.site_slogan || 'Kubernetes 异常监测\n与根因分析平台'}
          </Title>
          <Text style={{ color: 'rgba(255,255,255,0.4)', fontSize: 14, lineHeight: 1.8, display: 'block' }}>
            自动检测 Pod 运行异常，实时采集故障现场证据，
            关联多维上下文，帮助 SRE 快速定位根因。
          </Text>
          <div style={{ marginTop: 40, display: 'flex', gap: 28 }}>
            {[
              { label: '异常检测', icon: <AlertOutlined /> },
              { label: '证据采集', icon: <FileSearchOutlined /> },
              { label: '根因分析', icon: <BugOutlined /> },
            ].map((item) => (
              <div key={item.label} style={{ textAlign: 'center' }}>
                <div style={{
                  width: 40,
                  height: 40,
                  borderRadius: 10,
                  background: 'rgba(255,255,255,0.05)',
                  border: '1px solid rgba(255,255,255,0.08)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  margin: '0 auto 8px',
                  color: 'rgba(255,255,255,0.5)',
                  fontSize: 17,
                }}>
                  {item.icon}
                </div>
                <Text style={{ color: 'rgba(255,255,255,0.4)', fontSize: 12 }}>{item.label}</Text>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* 右侧登录表单 */}
      <div style={{
        width: 460,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        paddingRight: 40,
        position: 'relative',
        zIndex: 1,
      }}>
        <div style={{
          width: 380,
          padding: '44px 36px',
          background: 'rgba(255,255,255,0.04)',
          backdropFilter: 'blur(24px)',
          borderRadius: 16,
          border: '1px solid rgba(255,255,255,0.08)',
          boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
        }}>
          <Title level={4} style={{ color: '#fff', marginBottom: 6 }}>
            登录
          </Title>
          <Text style={{ color: 'rgba(255,255,255,0.35)', display: 'block', marginBottom: 28, fontSize: 13 }}>
            请输入账号和密码
          </Text>

          <Form onFinish={onFinish} size="large" autoComplete="off" className="login-dark-input">
            <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
              <Input
                prefix={<UserOutlined />}
                placeholder="用户名"
                style={{ borderRadius: 8, height: 44 }}
                autoFocus
              />
            </Form.Item>
            <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password
                prefix={<LockOutlined />}
                placeholder="密码"
                style={{ borderRadius: 8, height: 44 }}
              />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
              <Button
                type="primary"
                htmlType="submit"
                block
                loading={loading}
                style={{
                  height: 44,
                  borderRadius: 8,
                  fontSize: 14,
                  fontWeight: 500,
                  background: 'linear-gradient(135deg, #1890ff 0%, #096dd9 100%)',
                  border: 'none',
                  boxShadow: '0 4px 16px rgba(24,144,255,0.3)',
                }}
              >
                登 录
              </Button>
            </Form.Item>
          </Form>

          {ssoConfig?.enabled && (
            <>
              <Divider style={{ borderColor: 'rgba(255,255,255,0.1)', margin: '20px 0 16px' }}>
                <Text style={{ color: 'rgba(255,255,255,0.3)', fontSize: 12 }}>或</Text>
              </Divider>
              <Button
                block
                icon={<LoginOutlined />}
                loading={ssoLoading}
                onClick={onSSOLogin}
                style={{
                  height: 44,
                  borderRadius: 8,
                  fontSize: 14,
                  fontWeight: 500,
                  background: 'rgba(255,255,255,0.06)',
                  border: '1px solid rgba(255,255,255,0.12)',
                  color: 'rgba(255,255,255,0.85)',
                }}
              >
                {ssoConfig.providerName || 'SSO 单点登录'}
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
