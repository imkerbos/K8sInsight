import { Card, Form, Input, Button, message, Spin } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getSecuritySettings,
  updateSecuritySettings,
  type SecuritySettings,
} from '../../api/settings'
import { useEffect } from 'react'

export default function SecuritySettingsPage() {
  const [form] = Form.useForm()
  const queryClient = useQueryClient()

  const { data: settings, isLoading } = useQuery({
    queryKey: ['security-settings'],
    queryFn: getSecuritySettings,
  })

  useEffect(() => {
    if (settings) {
      form.setFieldsValue(settings)
    }
  }, [settings, form])

  const mutation = useMutation({
    mutationFn: (values: Partial<SecuritySettings>) => updateSecuritySettings(values),
    onSuccess: () => {
      message.success('安全设置已保存')
      queryClient.invalidateQueries({ queryKey: ['security-settings'] })
    },
    onError: () => {
      message.error('保存失败')
    },
  })

  const onFinish = (values: Record<string, string>) => {
    mutation.mutate(values)
  }

  return (
    <div>
      <div style={{ marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>安全配置</h2>
      </div>
      <Card
        style={{ border: '1px solid #f0f0f0', borderRadius: 8 }}
      >
        {isLoading ? (
          <Spin />
        ) : (
          <Form form={form} layout="vertical" onFinish={onFinish} style={{ maxWidth: 420 }}>
            <Form.Item
              name="security_min_password_length"
              label="密码最小长度"
              rules={[
                { required: true, message: '请输入密码最小长度' },
                {
                  validator: (_, value) => {
                    const n = Number(value)
                    if (!value || (Number.isInteger(n) && n >= 1)) return Promise.resolve()
                    return Promise.reject(new Error('请输入正整数'))
                  },
                },
              ]}
            >
              <Input type="number" min={1} placeholder="6" suffix="位" />
            </Form.Item>
            <Form.Item
              name="security_access_token_ttl"
              label="Access Token 有效期"
              rules={[{ required: true, message: '请输入有效期' }]}
              extra="示例: 15m, 1h, 2h"
            >
              <Input placeholder="2h" />
            </Form.Item>
            <Form.Item
              name="security_refresh_token_ttl"
              label="Refresh Token 有效期"
              rules={[{ required: true, message: '请输入有效期' }]}
              extra="示例: 24h, 168h, 720h"
            >
              <Input placeholder="168h" />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" loading={mutation.isPending}>
                保存
              </Button>
            </Form.Item>
          </Form>
        )}
      </Card>
    </div>
  )
}
