import { useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Typography } from 'antd'
import { AlertOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'

import { listAlerts } from '../api'

export default function DashboardPage() {
  const [totalAlerts, setTotalAlerts] = useState<number | null>(null)
  const [weekAlerts, setWeekAlerts] = useState<number | null>(null)
  const [todayAlerts, setTodayAlerts] = useState<number | null>(null)

  useEffect(() => {
    // 分页 total 简算（后端暂无统计接口）：累计不带时间条件
    listAlerts({ page: 1, page_size: 1 })
      .then((r) => setTotalAlerts(r.total))
      .catch(() => setTotalAlerts(null))
    listAlerts({ start: dayjs().startOf('week').toISOString(), page: 1, page_size: 1 })
      .then((r) => setWeekAlerts(r.total))
      .catch(() => setWeekAlerts(null))
    listAlerts({ start: dayjs().startOf('day').toISOString(), page: 1, page_size: 1 })
      .then((r) => setTodayAlerts(r.total))
      .catch(() => setTodayAlerts(null))
  }, [])

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        仪表盘
      </Typography.Title>
      <Row gutter={16}>
        <Col span={8}>
          <Card>
            <Statistic title="累计告警" value={totalAlerts ?? '-'} prefix={<AlertOutlined />} />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic title="本周告警" value={weekAlerts ?? '-'} prefix={<AlertOutlined />} />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic title="今日告警" value={todayAlerts ?? '-'} prefix={<AlertOutlined />} />
          </Card>
        </Col>
      </Row>
    </div>
  )
}
