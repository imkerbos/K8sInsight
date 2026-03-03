import client from './client'
import axios from 'axios'

export interface BrandingSettings {
  site_title: string
  site_logo: string
  site_slogan: string
}

export interface SecuritySettings {
  security_min_password_length: string
  security_access_token_ttl: string
  security_refresh_token_ttl: string
}

export interface NotifyTestRequest {
  channel: 'webhook' | 'lark' | 'telegram'
  incidentId?: string
  name?: string
  url?: string
  headers?: Record<string, string>
  secret?: string
  botToken?: string
  chatId?: string
  parseMode?: string
}

export interface NotifyWebhookSetting {
  name: string
  url: string
  headers?: Record<string, string>
}

export interface NotifyLarkSetting {
  name: string
  url: string
  secret?: string
}

export interface NotifyTelegramSetting {
  name: string
  botToken: string
  chatId: string
  parseMode?: string
}

export interface NotifySettings {
  enabled: boolean
  channel: 'webhook' | 'lark' | 'telegram'
  webhooks: NotifyWebhookSetting[]
  larks: NotifyLarkSetting[]
  telegrams: NotifyTelegramSetting[]
}

// 公开接口，无需认证（登录页也能调用）
export async function getBranding(): Promise<BrandingSettings> {
  const { data } = await axios.get<BrandingSettings>('/api/v1/settings/branding')
  return data
}

export async function updateBranding(settings: Partial<BrandingSettings>) {
  const { data } = await client.put('/settings/branding', settings)
  return data
}

export async function getSecuritySettings(): Promise<SecuritySettings> {
  const { data } = await client.get<SecuritySettings>('/settings/security')
  return data
}

export async function updateSecuritySettings(settings: Partial<SecuritySettings>) {
  const { data } = await client.put('/settings/security', settings)
  return data
}

export async function getNotifySettings(): Promise<NotifySettings> {
  const { data } = await client.get<NotifySettings>('/settings/notify')
  return data
}

export async function updateNotifySettings(settings: NotifySettings) {
  const { data } = await client.put('/settings/notify', settings)
  return data
}

export async function testNotify(request: NotifyTestRequest) {
  const { data } = await client.post('/settings/notify/test', request)
  return data
}

// SSO 设置 API

export interface SSOSettings {
  sso_enabled: string
  sso_provider_name: string
  sso_client_id: string
  sso_client_secret: string
  sso_issuer_url: string
  sso_redirect_uri: string
  sso_scopes: string
  sso_auto_create_user: string
  sso_default_role: string
}

export async function getSSOSettings(): Promise<SSOSettings> {
  const { data } = await client.get<SSOSettings>('/settings/sso')
  return data
}

export async function updateSSOSettings(settings: Partial<SSOSettings>) {
  const { data } = await client.put('/settings/sso', settings)
  return data
}
