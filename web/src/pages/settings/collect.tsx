import { Card, Form, Input, Button, message, Spin, Switch } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getCollectSettings,
  updateCollectSettings,
  type CollectSettings,
} from '../../api/settings'
import { useEffect } from 'react'

export default function CollectSettingsPage() {
  const [form] = Form.useForm()
  const queryClient = useQueryClient()

  const { data: settings, isLoading } = useQuery({
    queryKey: ['collect-settings'],
    queryFn: getCollectSettings,
  })

  useEffect(() => {
    if (settings) {
      form.setFieldsValue(settings)
    }
  }, [settings, form])

  const mutation = useMutation({
    mutationFn: (values: CollectSettings) => updateCollectSettings(values),
    onSuccess: () => {
      message.success('资源采集配置已保存')
      queryClient.invalidateQueries({ queryKey: ['collect-settings'] })
    },
    onError: () => {
      message.error('保存失败')
    },
  })

  const onFinish = (values: CollectSettings) => {
    mutation.mutate(values)
  }

  return (
    <div>
      <div style={{ marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>资源采集配置</h2>
      </div>
      <Card style={{ border: '1px solid #f0f0f0', borderRadius: 8 }}>
        {isLoading ? (
          <Spin />
        ) : (
          <Form form={form} layout="vertical" onFinish={onFinish} style={{ maxWidth: 720 }}>
            <Form.Item
              name="enableMetrics"
              label="启用资源指标采集"
              valuePropName="checked"
            >
              <Switch checkedChildren="启用" unCheckedChildren="禁用" />
            </Form.Item>
            <Form.Item
              name="prometheusURL"
              label="Prometheus 地址"
              extra='示例: http://kube-prometheus-stack-prometheus.monitoring.svc:9090'
            >
              <Input placeholder="http://kube-prometheus-stack-prometheus.monitoring.svc:9090" />
            </Form.Item>
            <Form.Item
              name="promQueryRange"
              label="Prometheus 查询时间窗"
              rules={[{ required: true, message: '请输入查询时间窗' }]}
              extra="示例: 2m, 10m, 1h"
            >
              <Input placeholder="10m" />
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
