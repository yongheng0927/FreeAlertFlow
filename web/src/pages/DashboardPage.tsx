import { useCallback, useEffect, useState } from 'react'
import { Button, Card, Col, Row, Statistic, Typography, message } from 'antd'
import { AlertOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'

import { listAlerts } from '../api'

type StatKey = 'total' | 'week' | 'today'

export default function DashboardPage() {
  const [totalAlerts, setTotalAlerts] = useState<number | null>(null)
  const [weekAlerts, setWeekAlerts] = useState<number | null>(null)
  const [todayAlerts, setTodayAlerts] = useState<number | null>(null)
  const [failed, setFailed] = useState<Record<StatKey, boolean>>({
    total: false,
    week: false,
    today: false,
  })

  const load = useCallback(() => {
    setFailed({ total: false, week: false, today: false })
    // 分页 total 简算（后端暂无统计接口）：累计不带时间条件
    const run = (key: StatKey, req: Promise<{ total: number }>, set: (v: number | null) => void) => {
      req.then((r) => set(r.total)).catch(() => {
        set(null)
        setFailed((f) => ({ ...f, [key]: true }))
        message.error('统计数据加载失败，可点击卡片上的重试')
      })
    }
    run('total', listAlerts({ page: 1, page_size: 1 }), setTotalAlerts)
    run(
      'week',
      listAlerts({ start: dayjs().startOf('week').toISOString(), page: 1, page_size: 1 }),
      setWeekAlerts,
    )
    run(
      'today',
      listAlerts({ start: dayjs().startOf('day').toISOString(), page: 1, page_size: 1 }),
      setTodayAlerts,
    )
  }, [])

  useEffect(() => {
    load()
  }, [load])

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        仪表盘
      </Typography.Title>
      <Row gutter={16}>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic title="累计告警" value={totalAlerts ?? '-'} prefix={<AlertOutlined />} />
            {failed.total && (
              <Button
                size="small"
                type="link"
                icon={<ReloadOutlined />}
                style={{ padding: 0 }}
                onClick={load}
              >
                重试
              </Button>
            )}
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic title="本周告警" value={weekAlerts ?? '-'} prefix={<AlertOutlined />} />
            {failed.week && (
              <Button
                size="small"
                type="link"
                icon={<ReloadOutlined />}
                style={{ padding: 0 }}
                onClick={load}
              >
                重试
              </Button>
            )}
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic title="今日告警" value={todayAlerts ?? '-'} prefix={<AlertOutlined />} />
            {failed.today && (
              <Button
                size="small"
                type="link"
                icon={<ReloadOutlined />}
                style={{ padding: 0 }}
                onClick={load}
              >
                重试
              </Button>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}
