import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Spin, Result, Button } from 'antd'
import { getSSOAuthorizeURL } from '../../api/auth'
import { useAuth } from '../../contexts/AuthContext'

/**
 * IdP-initiated SSO 入口页面
 * 当 IdP 将用户重定向到 /auth/sso/login 时，
 * 自动发起 OIDC 授权流程，跳转到 IdP 进行认证。
 * 由于用户在 IdP 已认证，IdP 会立即重定向回 /auth/sso/callback 完成登录。
 */
export default function SSOLoginPage() {
  const navigate = useNavigate()
  const { authenticated, loading } = useAuth()
  const [error, setError] = useState<string | null>(null)
  const calledRef = useRef(false)

  useEffect(() => {
    // 如果已登录，直接跳转首页
    if (authenticated) {
      navigate('/', { replace: true })
      return
    }
    // 等待 auth 状态加载完成
    if (loading) return

    if (calledRef.current) return
    calledRef.current = true

    getSSOAuthorizeURL()
      .then(({ authorizeUrl }) => {
        window.location.href = authorizeUrl
      })
      .catch(() => {
        setError('获取 SSO 登录地址失败，请检查 SSO 配置')
      })
  }, [authenticated, loading, navigate])

  if (error) {
    return (
      <div style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#f0f2f5',
      }}>
        <Result
          status="error"
          title="SSO 登录失败"
          subTitle={error}
          extra={
            <Button type="primary" onClick={() => navigate('/login', { replace: true })}>
              返回登录页
            </Button>
          }
        />
      </div>
    )
  }

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: 16,
      background: '#f0f2f5',
    }}>
      <Spin size="large" />
      <span style={{ color: '#666' }}>正在跳转到统一认证...</span>
    </div>
  )
}
