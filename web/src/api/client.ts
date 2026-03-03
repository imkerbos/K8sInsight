import axios from 'axios'
import { refreshToken as refreshTokenAPI } from './auth'

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

// 请求拦截器：自动携带 access token
client.interceptors.request.use((config) => {
  const token = localStorage.getItem('accessToken')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：401 时尝试刷新 token
let refreshing: Promise<string> | null = null

client.interceptors.response.use(
  (response) => response,
  async (error) => {
    const original = error.config
    if (error.response?.status !== 401 || original._retry) {
      return Promise.reject(error)
    }

    original._retry = true
    const storedRefreshToken = localStorage.getItem('refreshToken')

    if (!storedRefreshToken) {
      // 无 refresh token，跳转登录
      localStorage.removeItem('accessToken')
      localStorage.removeItem('refreshToken')
      window.location.href = '/login'
      return Promise.reject(error)
    }

    try {
      // 并发请求共享同一个刷新 promise
      if (!refreshing) {
        refreshing = refreshTokenAPI(storedRefreshToken).then((res) => {
          localStorage.setItem('accessToken', res.accessToken)
          localStorage.setItem('refreshToken', res.refreshToken)
          return res.accessToken
        }).finally(() => {
          refreshing = null
        })
      }

      const newToken = await refreshing
      original.headers.Authorization = `Bearer ${newToken}`
      return client(original)
    } catch {
      localStorage.removeItem('accessToken')
      localStorage.removeItem('refreshToken')
      window.location.href = '/login'
      return Promise.reject(error)
    }
  },
)

export default client
