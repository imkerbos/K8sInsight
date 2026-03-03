import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Spin, Result, Button } from 'antd'
import { ssoCallback } from '../../api/auth'
import { useAuth } from '../../contexts/AuthContext'

export default function SSOCallbackPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { loginWithSSO } = useAuth()
  const [error, setError] = useState<string | null>(null)
  const calledRef = useRef(false)

  const params = useMemo(() => ({
    code: searchParams.get('code'),
    state: searchParams.get('state'),
  }), [searchParams])

  useEffect(() => {
    if (calledRef.current) return
    calledRef.current = true

    const { code, state } = params

    if (!code || !state) {
      // defer setState to next microtask to avoid synchronous setState in effect
      Promise.resolve().then(() => setError('缺少必要的回调参数'))
      return
    }

    ssoCallback(code, state)
      .then(async (res) => {
        await loginWithSSO(res.accessToken, res.refreshToken)
        navigate('/', { replace: true })
      })
      .catch((err: unknown) => {
        const axiosErr = err as { response?: { data?: { error?: string } } }
        const msg = axiosErr?.response?.data?.error || 'SSO 登录失败'
        setError(msg)
      })
  }, [params, loginWithSSO, navigate])

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
      <span style={{ color: '#666' }}>正在完成 SSO 登录...</span>
    </div>
  )
}
