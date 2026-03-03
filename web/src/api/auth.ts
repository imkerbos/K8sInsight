import axios from 'axios'
import type { LoginRequest, TokenResponse, ChangePasswordRequest, User } from '../types/auth'

// 独立的 axios 实例，用于 auth 请求（不经过拦截器，避免循环）
const authClient = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

export async function login(req: LoginRequest): Promise<TokenResponse> {
  const { data } = await authClient.post<TokenResponse>('/auth/login', req)
  return data
}

export async function refreshToken(token: string): Promise<TokenResponse> {
  const { data } = await authClient.post<TokenResponse>('/auth/refresh', {
    refreshToken: token,
  })
  return data
}

export async function logout(token: string): Promise<void> {
  const accessToken = localStorage.getItem('accessToken')
  await authClient.post('/auth/logout', { refreshToken: token }, {
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
  })
}

export async function changePassword(req: ChangePasswordRequest): Promise<void> {
  const accessToken = localStorage.getItem('accessToken')
  await authClient.post('/auth/change-password', req, {
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
  })
}

export async function getMe(): Promise<User> {
  const accessToken = localStorage.getItem('accessToken')
  const { data } = await authClient.get<User>('/auth/me', {
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
  })
  return data
}

// SSO API

export interface SSOConfigResponse {
  enabled: boolean
  providerName: string
}

export async function getSSOConfig(): Promise<SSOConfigResponse> {
  const { data } = await authClient.get<SSOConfigResponse>('/auth/sso/config')
  return data
}

export async function getSSOAuthorizeURL(): Promise<{ authorizeUrl: string }> {
  const { data } = await authClient.get<{ authorizeUrl: string }>('/auth/sso/authorize')
  return data
}

export async function ssoCallback(code: string, state: string): Promise<TokenResponse> {
  const { data } = await authClient.post<TokenResponse>('/auth/sso/callback', { code, state })
  return data
}
