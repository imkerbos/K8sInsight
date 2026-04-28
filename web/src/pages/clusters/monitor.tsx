import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Card, Select, Spin, Empty, Typography, Space, Tag, Breadcrumb, Row, Col, Statistic } from 'antd'
import {
  ArrowLeftOutlined,
  ReloadOutlined,
  CloudServerOutlined,
} from '@ant-design/icons'
import { useMemo, useState, useCallback } from 'react'
import ReactECharts from 'echarts-for-react'
import dayjs from '../../utils/dayjs'
import { getClusterMetrics, listClusters } from '../../api/clusters'
import type { Cluster } from '../../types/cluster'
import './monitor.css'

const { Text } = Typography

const RANGE_OPTIONS = [
  { label: '最近 1 小时', value: '1h' },
  { label: '最近 6 小时', value: '6h' },
  { label: '最近 24 小时', value: '24h' },
  { label: '最近 3 天', value: '72h' },
]

const REFRESH_INTERVAL = 60_000 // 60s auto-refresh

/** 将 Prometheus [timestamp, value_string][] 转为 ECharts 可消费格式 */
function toChartData(series: [number, string][] | undefined) {
  if (!series?.length) return { times: [] as string[], values: [] as number[] }
  const times = series.map(([ts]) => dayjs.unix(ts).format('HH:mm'))
  const values = series.map(([, v]) => Number(v))
  return { times, values }
}

/** 取最新值 */
function latest(series: [number, string][] | undefined): number | null {
  if (!series?.length) return null
  return Number(series[series.length - 1][1])
}

/** 字节格式化 */
function fmtBytes(bytes: number | null): string {
  if (bytes === null || !Number.isFinite(bytes)) return '-'
  if (bytes < 1024) return bytes.toFixed(0) + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KiB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MiB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GiB'
}

/** 字节速率格式化 */
function fmtBytesPerSec(bps: number | null): string {
  if (bps === null || !Number.isFinite(bps)) return '-'
  if (bps < 1024) return bps.toFixed(0) + ' B/s'
  if (bps < 1024 * 1024) return (bps / 1024).toFixed(1) + ' KiB/s'
  return (bps / (1024 * 1024)).toFixed(1) + ' MiB/s'
}

/** 构建 ECharts 折线图 option */
function buildLineOption(
  times: string[],
  datasets: { name: string; values: number[]; color: string }[],
  yFormatter?: (v: number) => string,
) {
  return {
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(255,255,255,0.96)',
      borderColor: '#e8e8e8',
      textStyle: { color: '#333', fontSize: 12 },
    },
    grid: { left: 70, right: 20, top: 30, bottom: 36 },
    xAxis: {
      type: 'category',
      data: times,
      axisLabel: { fontSize: 10, color: '#8c8c8c', interval: Math.max(0, Math.floor(times.length / 8)) },
      axisLine: { lineStyle: { color: '#e8e8e8' } },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        fontSize: 10,
        color: '#8c8c8c',
        formatter: yFormatter,
      },
      splitLine: { lineStyle: { color: '#f0f0f0', type: 'dashed' } },
    },
    legend: {
      show: datasets.length > 1,
      top: 0,
      right: 0,
      textStyle: { fontSize: 12, color: '#595959' },
    },
    series: datasets.map((ds) => ({
      name: ds.name,
      type: 'line',
      data: ds.values,
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2, color: ds.color },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: ds.color + '30' },
            { offset: 1, color: ds.color + '05' },
          ],
        },
      },
    })),
  }
}

export default function ClusterMonitor() {
  const { id } = useParams<{ id: string }>()
  const [range, setRange] = useState('1h')

  // 集群基础信息
  const { data: clusters } = useQuery({
    queryKey: ['clusters'],
    queryFn: listClusters,
    staleTime: 60_000,
  })
  const cluster = useMemo<Cluster | undefined>(
    () => clusters?.find((c) => c.id === id),
    [clusters, id],
  )

  // 指标查询
  const {
    data: metrics,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ['cluster-metrics', id, range],
    queryFn: () => getClusterMetrics(id!, range),
    enabled: !!id,
    refetchInterval: REFRESH_INTERVAL,
    retry: 1,
  })

  // 手动刷新
  const handleRefresh = useCallback(() => {
    refetch()
  }, [refetch])

  // 提取数据
  const cpu = useMemo(() => toChartData(metrics?.series?.cpu_usage), [metrics])
  const mem = useMemo(() => toChartData(metrics?.series?.memory_usage), [metrics])
  const netRx = useMemo(() => toChartData(metrics?.series?.network_rx), [metrics])
  const netTx = useMemo(() => toChartData(metrics?.series?.network_tx), [metrics])
  const pods = useMemo(() => toChartData(metrics?.series?.pod_count), [metrics])

  // 概要统计
  const currentCPU = latest(metrics?.series?.cpu_usage)
  const currentMem = latest(metrics?.series?.memory_usage)
  const currentPods = latest(metrics?.series?.pod_count)
  const cpuReq = latest(metrics?.series?.cpu_requests)
  const memReq = latest(metrics?.series?.mem_requests)

  return (
    <div className="cluster-monitor-page">
      <Breadcrumb
        items={[
          { title: <Link to="/clusters">集群管理</Link> },
          { title: cluster?.name || id },
          { title: '监控面板' },
        ]}
        style={{ marginBottom: 16 }}
      />

      <div className="cluster-monitor-header">
        <Space align="center" size={12}>
          <Link to="/clusters"><ArrowLeftOutlined /></Link>
          <CloudServerOutlined style={{ fontSize: 20, color: '#1890ff' }} />
          <span className="cluster-monitor-title">{cluster?.name || '集群监控'}</span>
          {cluster && (
            <Tag color={cluster.connectionStatus === 'connected' ? 'green' : 'default'}>
              {cluster.connectionStatus === 'connected' ? '已连接' : cluster.connectionStatus}
            </Tag>
          )}
          {cluster?.version && <Text type="secondary">v{cluster.version}</Text>}
        </Space>
        <Space>
          <Select
            value={range}
            onChange={setRange}
            options={RANGE_OPTIONS}
            style={{ width: 140 }}
          />
          <a onClick={handleRefresh} style={{ fontSize: 16, color: '#1890ff', cursor: 'pointer' }}>
            <ReloadOutlined spin={isLoading} />
          </a>
        </Space>
      </div>

      {isLoading && !metrics && (
        <div style={{ textAlign: 'center', padding: 80 }}>
          <Spin size="large" tip="正在加载集群指标..." />
        </div>
      )}

      {isError && (
        <Card>
          <Empty
            description={
              <span>
                指标加载失败：{(error as Error)?.message || '请检查 Prometheus 配置'}
              </span>
            }
          />
        </Card>
      )}

      {metrics && (
        <>
          {/* 概要卡片 */}
          <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
            <Col xs={12} sm={8} lg={4}>
              <Card size="small" className="cluster-stat-card">
                <Statistic
                  title="CPU 使用"
                  value={currentCPU !== null ? currentCPU.toFixed(2) : '-'}
                  suffix="cores"
                  valueStyle={{ fontSize: 20 }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} lg={4}>
              <Card size="small" className="cluster-stat-card">
                <Statistic
                  title="CPU 请求"
                  value={cpuReq !== null ? cpuReq.toFixed(2) : '-'}
                  suffix="cores"
                  valueStyle={{ fontSize: 20, color: '#8c8c8c' }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} lg={4}>
              <Card size="small" className="cluster-stat-card">
                <Statistic
                  title="内存使用"
                  value={fmtBytes(currentMem)}
                  valueStyle={{ fontSize: 20 }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} lg={4}>
              <Card size="small" className="cluster-stat-card">
                <Statistic
                  title="内存请求"
                  value={fmtBytes(memReq)}
                  valueStyle={{ fontSize: 20, color: '#8c8c8c' }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} lg={4}>
              <Card size="small" className="cluster-stat-card">
                <Statistic
                  title="Pod 数量"
                  value={currentPods !== null ? Math.round(currentPods) : '-'}
                  valueStyle={{ fontSize: 20 }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} lg={4}>
              <Card size="small" className="cluster-stat-card">
                <Statistic
                  title="节点数"
                  value={cluster?.nodeCount ?? '-'}
                  valueStyle={{ fontSize: 20 }}
                />
              </Card>
            </Col>
          </Row>

          {/* CPU 趋势 */}
          <Row gutter={[16, 16]}>
            <Col xs={24} lg={12}>
              <Card title="CPU 使用趋势 (cores)" size="small" className="cluster-chart-card">
                {cpu.values.length > 0 ? (
                  <ReactECharts
                    option={buildLineOption(
                      cpu.times,
                      [{ name: 'CPU Usage', values: cpu.values, color: '#1890ff' }],
                      (v: number) => v.toFixed(2),
                    )}
                    style={{ height: 240 }}
                    notMerge
                    lazyUpdate
                  />
                ) : (
                  <Empty description="暂无 CPU 数据" style={{ padding: 40 }} />
                )}
              </Card>
            </Col>

            {/* 内存趋势 */}
            <Col xs={24} lg={12}>
              <Card title="内存使用趋势" size="small" className="cluster-chart-card">
                {mem.values.length > 0 ? (
                  <ReactECharts
                    option={buildLineOption(
                      mem.times,
                      [{ name: 'Memory Working Set', values: mem.values, color: '#52c41a' }],
                      (v: number) => fmtBytes(v),
                    )}
                    style={{ height: 240 }}
                    notMerge
                    lazyUpdate
                  />
                ) : (
                  <Empty description="暂无内存数据" style={{ padding: 40 }} />
                )}
              </Card>
            </Col>

            {/* 网络 I/O */}
            <Col xs={24} lg={12}>
              <Card title="网络 I/O 趋势" size="small" className="cluster-chart-card">
                {netRx.values.length > 0 || netTx.values.length > 0 ? (
                  <ReactECharts
                    option={buildLineOption(
                      netRx.times.length > 0 ? netRx.times : netTx.times,
                      [
                        { name: '接收 (RX)', values: netRx.values, color: '#13c2c2' },
                        { name: '发送 (TX)', values: netTx.values, color: '#fa8c16' },
                      ],
                      (v: number) => fmtBytesPerSec(v),
                    )}
                    style={{ height: 240 }}
                    notMerge
                    lazyUpdate
                  />
                ) : (
                  <Empty description="暂无网络数据" style={{ padding: 40 }} />
                )}
              </Card>
            </Col>

            {/* Pod 数量趋势 */}
            <Col xs={24} lg={12}>
              <Card title="Pod 数量趋势" size="small" className="cluster-chart-card">
                {pods.values.length > 0 ? (
                  <ReactECharts
                    option={buildLineOption(
                      pods.times,
                      [{ name: 'Pods', values: pods.values, color: '#722ed1' }],
                      (v: number) => Math.round(v).toString(),
                    )}
                    style={{ height: 240 }}
                    notMerge
                    lazyUpdate
                  />
                ) : (
                  <Empty description="暂无 Pod 数据" style={{ padding: 40 }} />
                )}
              </Card>
            </Col>
          </Row>
        </>
      )}
    </div>
  )
}
