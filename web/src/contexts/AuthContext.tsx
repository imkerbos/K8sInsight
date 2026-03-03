import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'
import type { User } from '../types/auth'
import { login as loginAPI, logout as logoutAPI, getMe } from '../api/auth'

interface AuthState {
  user: User | null
  permissions: string[]
  loading: boolean
  authenticated: boolean
}

interface AuthContextType extends AuthState {
  login: (username: string, password: string) => Promise<void>
  loginWithSSO: (accessToken: string, refreshToken: string) => Promise<void>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    permissions: [],
    loading: true,
    authenticated: false,
  })

  // 启动时检查是否已有有效 token
  useEffect(() => {
    const token = localStorage.getItem('accessToken')
    if (!token) {
      setState({ user: null, permissions: [], loading: false, authenticated: false })
      return
    }

    getMe()
      .then((user) => {
        setState({ user, permissions: user.permissions ?? [], loading: false, authenticated: true })
      })
      .catch(() => {
        localStorage.removeItem('accessToken')
        localStorage.removeItem('refreshToken')
        setState({ user: null, permissions: [], loading: false, authenticated: false })
      })
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const res = await loginAPI({ username, password })
    localStorage.setItem('accessToken', res.accessToken)
    localStorage.setItem('refreshToken', res.refreshToken)

    const user = await getMe()
    setState({ user, permissions: user.permissions ?? [], loading: false, authenticated: true })
  }, [])

  const loginWithSSO = useCallback(async (accessToken: string, refreshToken: string) => {
    localStorage.setItem('accessToken', accessToken)
    localStorage.setItem('refreshToken', refreshToken)

    const user = await getMe()
    setState({ user, permissions: user.permissions ?? [], loading: false, authenticated: true })
  }, [])

  const logout = useCallback(async () => {
    const rt = localStorage.getItem('refreshToken')
    if (rt) {
      try {
        await logoutAPI(rt)
      } catch {
        // 忽略登出错误
      }
    }
    localStorage.removeItem('accessToken')
    localStorage.removeItem('refreshToken')
    setState({ user: null, permissions: [], loading: false, authenticated: false })
  }, [])

  const refreshUser = useCallback(async () => {
    try {
      const user = await getMe()
      setState((prev) => ({ ...prev, user, permissions: user.permissions ?? [] }))
    } catch {
      // 忽略
    }
  }, [])

  return (
    <AuthContext.Provider value={{ ...state, login, loginWithSSO, logout, refreshUser }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return ctx
}
