import { Card, Form, Input, Button, message, Spin, Select, Switch } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  testNotify,
  getNotifySettings,
  updateNotifySettings,
  type NotifyTestRequest,
  type NotifySettings,
} from '../../api/settings'
import { useEffect } from 'react'

export default function NotifySettingsPage() {
  const [notifyConfigForm] = Form.useForm()
  const queryClient = useQueryClient()

  const { data: notifySettings, isLoading: isNotifyLoading } = useQuery({
    queryKey: ['notify-settings'],
    queryFn: getNotifySettings,
  })

  useEffect(() => {
    if (!notifySettings) return
    notifyConfigForm.setFieldsValue({
      enabled: notifySettings.enabled,
      channel: notifySettings.channel || 'webhook',
      webhookName: notifySettings.webhooks[0]?.name ?? 'ops-webhook',
      webhookURL: notifySettings.webhooks[0]?.url ?? '',
      webhookHeaders: notifySettings.webhooks[0]?.headers ? JSON.stringify(notifySettings.webhooks[0].headers, null, 2) : '',
      larkName: notifySettings.larks[0]?.name ?? 'ops-lark',
      larkURL: notifySettings.larks[0]?.url ?? '',
      larkSecret: notifySettings.larks[0]?.secret ?? '',
      telegramName: notifySettings.telegrams[0]?.name ?? 'ops-telegram',
      telegramBotToken: notifySettings.telegrams[0]?.botToken ?? '',
      telegramChatID: notifySettings.telegrams[0]?.chatId ?? '',
      telegramParseMode: notifySettings.telegrams[0]?.parseMode ?? 'HTML',
    })
  }, [notifySettings, notifyConfigForm])

  const testNotifyMutation = useMutation({
    mutationFn: (payload: NotifyTestRequest) => testNotify(payload),
    onSuccess: () => message.success('测试通知发送成功'),
    onError: (error: unknown) => {
      const axiosErr = error as { response?: { data?: { error?: string } } }
      message.error(axiosErr?.response?.data?.error || '测试通知发送失败')
    },
  })

  const notifyConfigMutation = useMutation({
    mutationFn: (payload: NotifySettings) => updateNotifySettings(payload),
    onSuccess: () => {
      message.success('通知配置已保存')
      queryClient.invalidateQueries({ queryKey: ['notify-settings'] })
    },
    onError: (error: unknown) => {
      const axiosErr = error as { response?: { data?: { error?: string } } }
      message.error(axiosErr?.response?.data?.error || '通知配置保存失败')
    },
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- form values are dynamic
  const onSaveNotifyConfig = (values: Record<string, any>) => {
    let webhookHeaders: Record<string, string> | undefined
    if (values.webhookHeaders) {
      try {
        webhookHeaders = JSON.parse(values.webhookHeaders)
      } catch {
        message.error('Webhook Headers 必须是合法 JSON')
        return
      }
    }

    const channel = (values.channel || 'webhook') as NotifySettings['channel']
    const payload: NotifySettings = {
      enabled: !!values.enabled,
      channel,
      webhooks: channel === 'webhook' && values.webhookURL
        ? [
            {
              name: values.webhookName || 'ops-webhook',
              url: values.webhookURL,
              headers: webhookHeaders,
            },
          ]
        : [],
      larks: channel === 'lark' && values.larkURL
        ? [
            {
              name: values.larkName || 'ops-lark',
              url: values.larkURL,
              secret: values.larkSecret || '',
            },
          ]
        : [],
      telegrams: channel === 'telegram' && values.telegramBotToken && values.telegramChatID
        ? [
            {
              name: values.telegramName || 'ops-telegram',
              botToken: values.telegramBotToken,
              chatId: values.telegramChatID,
              parseMode: values.telegramParseMode || 'HTML',
            },
          ]
        : [],
    }
    notifyConfigMutation.mutate(payload)
  }

  const onTestCurrentNotifyConfig = async () => {
    const values = await notifyConfigForm.validateFields()
    const channel = (values.channel || 'webhook') as NotifyTestRequest['channel']
    const payload: NotifyTestRequest = { channel }
    if (values.testIncidentId) {
      payload.incidentId = values.testIncidentId
    }
    if (channel === 'webhook') {
      payload.name = values.webhookName
      payload.url = values.webhookURL
      if (values.webhookHeaders) {
        try {
          payload.headers = JSON.parse(values.webhookHeaders)
        } catch {
          message.error('Webhook Headers 必须是合法 JSON')
          return
        }
      }
    } else if (channel === 'lark') {
      payload.name = values.larkName
      payload.url = values.larkURL
      payload.secret = values.larkSecret
    } else {
      payload.name = values.telegramName
      payload.botToken = values.telegramBotToken
      payload.chatId = values.telegramChatID
      payload.parseMode = values.telegramParseMode || 'HTML'
    }
    testNotifyMutation.mutate(payload)
  }

  return (
    <div>
      <div style={{ marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>通知配置</h2>
      </div>
      <Card
        style={{ border: '1px solid #f0f0f0', borderRadius: 8 }}
      >
        {isNotifyLoading ? (
          <Spin />
        ) : (
          <Form form={notifyConfigForm} layout="vertical" onFinish={onSaveNotifyConfig} style={{ maxWidth: 760 }} initialValues={{ enabled: false, channel: 'webhook', telegramParseMode: 'HTML' }}>
            <Form.Item name="enabled" label="启用通知" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="channel" label="通知通道（三选一）" rules={[{ required: true, message: '请选择通知通道' }]}>
              <Select options={[{ label: 'Webhook', value: 'webhook' }, { label: 'Lark(飞书卡片)', value: 'lark' }, { label: 'Telegram', value: 'telegram' }]} />
            </Form.Item>

            <Form.Item noStyle shouldUpdate>
              {({ getFieldValue }) => {
                const channel = getFieldValue('channel')
                if (channel === 'lark') {
                  return (
                    <>
                      <Form.Item label="Lark 名称" name="larkName">
                        <Input placeholder="ops-lark" />
                      </Form.Item>
                      <Form.Item label="Lark Webhook URL" name="larkURL">
                        <Input placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx" />
                      </Form.Item>
                      <Form.Item label="Lark Secret" name="larkSecret">
                        <Input.Password placeholder="可选" />
                      </Form.Item>
                    </>
                  )
                }
                if (channel === 'telegram') {
                  return (
                    <>
                      <Form.Item label="Telegram 名称" name="telegramName">
                        <Input placeholder="ops-telegram" />
                      </Form.Item>
                      <Form.Item label="Telegram Bot Token" name="telegramBotToken">
                        <Input.Password placeholder="123456:ABCDEF" />
                      </Form.Item>
                      <Form.Item label="Telegram Chat ID" name="telegramChatID">
                        <Input placeholder="-1001234567890" />
                      </Form.Item>
                      <Form.Item label="Telegram Parse Mode" name="telegramParseMode">
                        <Select options={[{ label: 'HTML', value: 'HTML' }, { label: 'MarkdownV2', value: 'MarkdownV2' }]} />
                      </Form.Item>
                    </>
                  )
                }
                return (
                  <>
                    <Form.Item label="Webhook 名称" name="webhookName">
                      <Input placeholder="ops-webhook" />
                    </Form.Item>
                    <Form.Item label="Webhook URL" name="webhookURL">
                      <Input placeholder="https://example.com/webhook" />
                    </Form.Item>
                    <Form.Item label="Webhook Headers(JSON)" name="webhookHeaders">
                      <Input.TextArea rows={4} placeholder='{"Authorization":"Bearer xxx"}' />
                    </Form.Item>
                  </>
                )
              }}
            </Form.Item>
            <Form.Item label="测试事件ID（可选，回放现有事件证据）" name="testIncidentId">
              <Input placeholder="如: d7993dc1-dd3a-44f9-a6d8-76c68fbd2455" />
            </Form.Item>

            <Form.Item style={{ marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" loading={notifyConfigMutation.isPending}>
                保存通知配置
              </Button>
              <Button style={{ marginLeft: 8 }} onClick={onTestCurrentNotifyConfig} loading={testNotifyMutation.isPending}>
                测试当前配置
              </Button>
            </Form.Item>
          </Form>
        )}
      </Card>
    </div>
  )
}
