import { Card, Typography, Space, Tag, Divider } from 'antd'
import {
  GithubOutlined,
  EyeOutlined,
  ThunderboltOutlined,
  ApartmentOutlined,
  ClusterOutlined,
  AlertOutlined,
  FileSearchOutlined,
} from '@ant-design/icons'

const { Title, Paragraph, Text, Link } = Typography

const features = [
  { icon: <EyeOutlined />, title: '异常检测', desc: 'Pod/容器状态持续监测，自动识别重启、OOMKill、异常退出、状态振荡等问题' },
  { icon: <ThunderboltOutlined />, title: '现场采集', desc: '故障发生时自动采集运行时证据：元数据、退出状态、重启历史、事件、资源用量' },
  { icon: <FileSearchOutlined />, title: '根因分析', desc: '多维关联分析，回答"为什么出了问题"而不仅仅是"出了问题"' },
  { icon: <ApartmentOutlined />, title: '时间线归档', desc: '事件归档与时间线构建，支持跨事件比较与事后复盘' },
  { icon: <ClusterOutlined />, title: '多维视图', desc: '集群、命名空间、应用多维度关联视图，全局掌握异常分布' },
  { icon: <AlertOutlined />, title: '通知集成', desc: '对接告警、工单、自动化等外部系统，形成闭环处置流程' },
]

const techStack = [
  { label: '后端', items: ['Go', 'Gin', 'client-go', 'SQLite'] },
  { label: '前端', items: ['React', 'TypeScript', 'Ant Design', 'Vite'] },
  { label: '部署', items: ['Docker Compose', 'Nginx'] },
]

export default function AboutPage() {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      padding: '48px 24px',
      minHeight: '100%',
    }}>
      {/* Header */}
      <div style={{ textAlign: 'center', marginBottom: 40 }}>
        <div style={{
          width: 72,
          height: 72,
          borderRadius: 16,
          background: 'linear-gradient(135deg, #1890ff 0%, #722ed1 100%)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          margin: '0 auto 20px',
          boxShadow: '0 8px 24px rgba(24, 144, 255, 0.25)',
        }}>
          <ClusterOutlined style={{ fontSize: 36, color: '#fff' }} />
        </div>
        <Title level={2} style={{ margin: 0, fontWeight: 700 }}>K8sInsight</Title>
        <Paragraph style={{ color: '#8c8c8c', fontSize: 15, marginTop: 8, marginBottom: 0 }}>
          Kubernetes 异常监测与根因分析平台
        </Paragraph>
      </div>

      {/* 核心理念 */}
      <Card
        style={{ width: '100%', maxWidth: 800, border: '1px solid #f0f0f0', borderRadius: 12, marginBottom: 24 }}
        styles={{ body: { padding: '24px 28px' } }}
      >
        <Title level={5} style={{ marginTop: 0, marginBottom: 12 }}>设计理念</Title>
        <Paragraph style={{ color: '#595959', marginBottom: 0, lineHeight: 1.8 }}>
          K8sInsight 不是监控系统，不是指标/日志平台，也不是传统告警工具。
          它专注于<Text strong>可解释性</Text>——帮助 SRE/运维人员理解故障<Text strong>为什么</Text>发生，
          而不只是<Text strong>发生了什么</Text>。通过在故障瞬间自动采集运行时证据、关联多维上下文，
          提供统一的问题视图，让每一次故障都能被复盘和理解。
        </Paragraph>
      </Card>

      {/* 核心能力 */}
      <Card
        style={{ width: '100%', maxWidth: 800, border: '1px solid #f0f0f0', borderRadius: 12, marginBottom: 24 }}
        styles={{ body: { padding: '24px 28px' } }}
      >
        <Title level={5} style={{ marginTop: 0, marginBottom: 16 }}>核心能力</Title>
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))',
          gap: 16,
        }}>
          {features.map((f) => (
            <div key={f.title} style={{
              display: 'flex',
              gap: 12,
              padding: '12px 14px',
              borderRadius: 8,
              background: '#fafafa',
            }}>
              <div style={{
                width: 36,
                height: 36,
                borderRadius: 8,
                background: '#e6f7ff',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
                color: '#1890ff',
                fontSize: 16,
              }}>
                {f.icon}
              </div>
              <div>
                <Text strong style={{ fontSize: 13 }}>{f.title}</Text>
                <Paragraph style={{ fontSize: 12, color: '#8c8c8c', marginBottom: 0, marginTop: 2 }}>
                  {f.desc}
                </Paragraph>
              </div>
            </div>
          ))}
        </div>
      </Card>

      {/* 技术栈 & 信息 */}
      <Card
        style={{ width: '100%', maxWidth: 800, border: '1px solid #f0f0f0', borderRadius: 12, marginBottom: 24 }}
        styles={{ body: { padding: '24px 28px' } }}
      >
        <Title level={5} style={{ marginTop: 0, marginBottom: 16 }}>技术栈</Title>
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {techStack.map((group) => (
            <div key={group.label} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <Text style={{ color: '#8c8c8c', width: 40, flexShrink: 0, fontSize: 13 }}>{group.label}</Text>
              <Space size={[6, 6]} wrap>
                {group.items.map((item) => (
                  <Tag key={item} style={{ margin: 0 }}>{item}</Tag>
                ))}
              </Space>
            </div>
          ))}
        </Space>

        <Divider style={{ margin: '20px 0' }} />

        <Space size={24}>
          <Link href="https://github.com/imkerbos/k8sinsight" target="_blank">
            <GithubOutlined style={{ marginRight: 4 }} />
            GitHub
          </Link>
          <Text style={{ color: '#8c8c8c', fontSize: 13 }}>MIT License</Text>
        </Space>
      </Card>
    </div>
  )
}
