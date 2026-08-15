import { useEffect, useState } from 'react'
import { Card, Col, List, Row, Statistic, Typography } from 'antd'
import { AlertOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import axios from 'axios'

import { listAlerts, listDeliveries } from '../api'
import { useAuth } from '../auth/useAuth'

interface TopChannel {
  name: string
  count: number
}

export default function DashboardPage() {
  const { canWrite } = useAuth()
  const [todayAlerts, setTodayAlerts] = useState<number | null>(null)
  const [successRate, setSuccessRate] = useState<number | null>(null)
  const [topFailed, setTopFailed] = useState<TopChannel[]>([])
  const [deliveryDenied, setDeliveryDenied] = useState(false)

  useEffect(() => {
    const start = dayjs().startOf('day').toISOString()

    // 今日告警数：分页 total 简算（后端暂无统计接口）
    listAlerts({ start, page: 1, page_size: 1 })
      .then((r) => setTodayAlerts(r.total))
      .catch(() => setTodayAlerts(null))

    if (!canWrite) {
      // GET /deliveries 是 editor/admin 专属，viewer 直接跳过
      setDeliveryDenied(true)
      return
    }

    // 投递成功率：成功/失败两个分页 total 简算
    Promise.all([
      listDeliveries({ start, status: 'success', page: 1, page_size: 1 }),
      listDeliveries({ start, status: 'failed', page: 1, page_size: 1 }),
    ])
      .then(([ok, bad]) => {
        const total = ok.total + bad.total
        setSuccessRate(total === 0 ? 100 : Math.round((ok.total / total) * 1000) / 10)
      })
      .catch(() => setSuccessRate(null))

    // 失败 Top 渠道：拉取今日失败投递（上限 100 条）按渠道聚合
    listDeliveries({ start, status: 'failed', page: 1, page_size: 100 })
      .then((r) => {
        const counter = new Map<string, number>()
        for (const d of r.list) {
          counter.set(d.channel_name, (counter.get(d.channel_name) ?? 0) + 1)
        }
        const top = [...counter.entries()]
          .map(([name, count]) => ({ name, count }))
          .sort((a, b) => b.count - a.count)
          .slice(0, 5)
        setTopFailed(top)
      })
      .catch((err) => {
        if (axios.isAxiosError(err) && err.response?.status === 403) setDeliveryDenied(true)
      })
  }, [canWrite])

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        仪表盘
      </Typography.Title>
      <Row gutter={16}>
        <Col span={8}>
          <Card>
            <Statistic
              title="今日告警数"
              value={todayAlerts ?? '-'}
              prefix={<AlertOutlined />}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="今日投递成功率"
              value={deliveryDenied ? '无权限' : successRate === null ? '-' : successRate}
              suffix={deliveryDenied || successRate === null ? undefined : '%'}
              prefix={<CheckCircleOutlined />}
              valueStyle={
                successRate !== null && successRate < 100 ? { color: '#cf1322' } : undefined
              }
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card title="今日失败 Top 渠道" styles={{ body: { paddingTop: 12 } }}>
            {deliveryDenied ? (
              <Typography.Text type="secondary">viewer 角色无投递数据权限</Typography.Text>
            ) : topFailed.length === 0 ? (
              <Typography.Text type="secondary">
                <CloseCircleOutlined style={{ marginRight: 8 }} />
                今日暂无失败投递
              </Typography.Text>
            ) : (
              <List
                size="small"
                dataSource={topFailed}
                renderItem={(item, idx) => (
                  <List.Item style={{ padding: '6px 0' }}>
                    <Typography.Text>
                      {idx + 1}. {item.name}
                    </Typography.Text>
                    <Typography.Text type="danger">{item.count} 次失败</Typography.Text>
                  </List.Item>
                )}
              />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}
