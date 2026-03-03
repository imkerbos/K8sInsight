import { Card, Form, Input, Button, message, Spin, Select, Switch } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getSSOSettings,
  updateSSOSettings,
  type SSOSettings,
} from '../../api/settings'
import { useEffect } from 'react'

export default function SSOSettingsPage() {
  const [ssoForm] = Form.useForm()
  const queryClient = useQueryClient()

  const { data: ssoSettings, isLoading: isSSOLoading } = useQuery({
    queryKey: ['sso-settings'],
    queryFn: getSSOSettings,
  })

  useEffect(() => {
    if (!ssoSettings) return
    ssoForm.setFieldsValue({
      sso_enabled: ssoSettings.sso_enabled === 'true',
      sso_provider_name: ssoSettings.sso_provider_name,
      sso_client_id: ssoSettings.sso_client_id,
      sso_client_secret: ssoSettings.sso_client_secret,
      sso_issuer_url: ssoSettings.sso_issuer_url,
      sso_redirect_uri: ssoSettings.sso_redirect_uri,
      sso_scopes: ssoSettings.sso_scopes,
      sso_auto_create_user: ssoSettings.sso_auto_create_user === 'true',
      sso_default_role: ssoSettings.sso_default_role || 'viewer',
    })
  }, [ssoSettings, ssoForm])

  const ssoMutation = useMutation({
    mutationFn: (values: Partial<SSOSettings>) => updateSSOSettings(values),
    onSuccess: () => {
      message.success('SSO 配置已保存')
      queryClient.invalidateQueries({ queryKey: ['sso-settings'] })
      queryClient.invalidateQueries({ queryKey: ['sso-config'] })
    },
    onError: (error: unknown) => {
      const axiosErr = error as { response?: { data?: { error?: string } } }
      message.error(axiosErr?.response?.data?.error || 'SSO 配置保存失败')
    },
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- form values are dynamic
  const onSaveSSO = (values: Record<string, any>) => {
    const payload: Partial<SSOSettings> = {
      sso_enabled: values.sso_enabled ? 'true' : 'false',
      sso_provider_name: values.sso_provider_name || '',
      sso_client_id: values.sso_client_id || '',
      sso_client_secret: values.sso_client_secret || '',
      sso_issuer_url: values.sso_issuer_url || '',
      sso_redirect_uri: values.sso_redirect_uri || '',
      sso_scopes: values.sso_scopes || 'openid,profile,email',
      sso_auto_create_user: values.sso_auto_create_user ? 'true' : 'false',
      sso_default_role: values.sso_default_role || 'viewer',
    }
    ssoMutation.mutate(payload)
  }

  return (
    <div>
      <div style={{ marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>SSO 单点登录</h2>
      </div>
      <Card
        style={{ border: '1px solid #f0f0f0', borderRadius: 8 }}
      >
        {isSSOLoading ? (
          <Spin />
        ) : (
          <Form form={ssoForm} layout="vertical" onFinish={onSaveSSO} style={{ maxWidth: 560 }} initialValues={{ sso_enabled: false, sso_auto_create_user: true, sso_default_role: 'viewer', sso_scopes: 'openid,profile,email' }}>
            <Form.Item name="sso_enabled" label="启用 SSO" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="sso_provider_name" label="Provider 显示名称" extra={'登录页按钮文字，如「企业统一认证」'}>
              <Input placeholder="SSO 单点登录" />
            </Form.Item>
            <Form.Item name="sso_client_id" label="Client ID">
              <Input placeholder="OIDC Client ID" />
            </Form.Item>
            <Form.Item name="sso_client_secret" label="Client Secret">
              <Input.Password placeholder="OIDC Client Secret" />
            </Form.Item>
            <Form.Item name="sso_issuer_url" label="Issuer URL" extra="OIDC Provider 的 Issuer URL，如 https://keycloak.example.com/realms/myrealm">
              <Input placeholder="https://idp.example.com" />
            </Form.Item>
            <Form.Item name="sso_redirect_uri" label="Redirect URI" extra="回调地址，通常为 http(s)://your-domain/auth/sso/callback">
              <Input placeholder="https://k8sinsight.example.com/auth/sso/callback" />
            </Form.Item>
            <Form.Item name="sso_scopes" label="Scopes" extra="逗号分隔，默认 openid,profile,email">
              <Input placeholder="openid,profile,email" />
            </Form.Item>
            <Form.Item name="sso_auto_create_user" label="自动创建用户" valuePropName="checked" extra="SSO 用户首次登录时自动创建本地账户">
              <Switch />
            </Form.Item>
            <Form.Item name="sso_default_role" label="新用户默认角色">
              <Select options={[
                { label: 'viewer (只读)', value: 'viewer' },
                { label: 'operator (运维)', value: 'operator' },
                { label: 'admin (管理员)', value: 'admin' },
              ]} />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" loading={ssoMutation.isPending}>
                保存 SSO 配置
              </Button>
            </Form.Item>
          </Form>
        )}
      </Card>
    </div>
  )
}
