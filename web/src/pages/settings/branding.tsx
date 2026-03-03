import { Card, Form, Input, Button, message, Spin, Avatar } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getBranding, updateBranding, type BrandingSettings } from '../../api/settings'
import { useEffect } from 'react'

export default function BrandingSettingsPage() {
  const [form] = Form.useForm()
  const queryClient = useQueryClient()

  const { data: branding, isLoading } = useQuery({
    queryKey: ['branding'],
    queryFn: getBranding,
  })

  useEffect(() => {
    if (branding) {
      form.setFieldsValue(branding)
    }
  }, [branding, form])

  const mutation = useMutation({
    mutationFn: (values: Partial<BrandingSettings>) => updateBranding(values),
    onSuccess: () => {
      message.success('产品设置已保存')
      queryClient.invalidateQueries({ queryKey: ['branding'] })
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
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>产品设置</h2>
      </div>
      <Card
        title={<span style={{ fontSize: 14, fontWeight: 500 }}>品牌信息</span>}
        style={{ border: '1px solid #f0f0f0', borderRadius: 8 }}
        styles={{ header: { borderBottom: '1px solid #f5f5f5' } }}
      >
        {isLoading ? (
          <Spin />
        ) : (
          <Form form={form} layout="vertical" onFinish={onFinish} style={{ maxWidth: 420 }}>
            <Form.Item name="site_title" label="产品名称">
              <Input placeholder="K8sInsight" />
            </Form.Item>
            <Form.Item name="site_logo" label="Logo URL">
              <Input placeholder="https://example.com/logo.png" />
            </Form.Item>
            {branding?.site_logo && (
              <div style={{ marginBottom: 16, marginTop: -8 }}>
                <span style={{ color: '#8c8c8c', fontSize: 12, marginRight: 8 }}>预览：</span>
                <Avatar src={branding.site_logo} shape="square" size={32} />
              </div>
            )}
            <Form.Item name="site_slogan" label="Slogan">
              <Input placeholder="Kubernetes 异常监测与根因分析" />
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
