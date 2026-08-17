import { useCallback, useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Button, Card, Col, Divider, Empty, List, Row, Statistic, Tag, Tooltip, Typography, message } from 'antd'
import { ArrowRightOutlined, CheckCircleFilled, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

import { getDashboardStats, listSources } from '../api'
import { errorMessage } from '../api/client'
import type { DashboardStats, Source } from '../api/types'
import { formatTime } from '../utils'

dayjs.extend(relativeTime)

const cardBodyStyle: CSSProperties = { minHeight: 116 }

/**  attention 卡片底部的小字链接/状态，统一样式 */
function CardFoot({ children }: { children: ReactNode }) {
  return <div style={{ marginTop: 8, fontSize: 13 }}>{children}</div>
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [sources, setSources] = useState<Source[] | null>(null)

  const load = useCallback(() => {
    getDashboardStats()
      .then(setStats)
      .catch((err) => message.error(errorMessage(err, '统计数据加载失败')))
    listSources()
      .then((r) => setSources(r.list))
      .catch((err) => message.error(errorMessage(err, '接入源状态加载失败')))
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const failed = stats?.failed_deliveries_today ?? 0
  const unmatched = stats?.unmatched_alerts_today ?? 0

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0, flex: 1 }}>
          仪表盘
        </Typography.Title>
        <Button icon={<ReloadOutlined />} onClick={load}>
          刷新
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Card title="告警量" styles={{ body: cardBodyStyle }}>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <Statistic title="今日" value={stats?.alerts_today ?? '-'} />
              <Divider type="vertical" style={{ height: 48, margin: '0 32px' }} />
              <Statistic title="本周" value={stats?.alerts_week ?? '-'} />
              <Divider type="vertical" style={{ height: 48, margin: '0 32px' }} />
              <Statistic title="累计" value={stats?.alerts_total ?? '-'} />
            </div>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card title="今日投递失败" styles={{ body: cardBodyStyle }}>
            <Statistic
              value={stats ? failed : '-'}
              valueStyle={{ color: failed > 0 ? '#cf1322' : '#3f8600', fontWeight: 600 }}
            />
            <CardFoot>
              {failed > 0 ? (
                <Link to="/deliveries">
                  查看失败投递 <ArrowRightOutlined />
                </Link>
              ) : (
                <Typography.Text type="success">
                  <CheckCircleFilled /> 全部投递成功
                </Typography.Text>
              )}
            </CardFoot>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card title="今日未匹配路由" styles={{ body: cardBodyStyle }}>
            <Statistic
              value={stats ? unmatched : '-'}
              valueStyle={{ color: unmatched > 0 ? '#d48806' : '#3f8600', fontWeight: 600 }}
            />
            <CardFoot>
              {unmatched > 0 ? (
                <Link to="/rules">
                  检查路由规则 <ArrowRightOutlined />
                </Link>
              ) : (
                <Typography.Text type="success">
                  <CheckCircleFilled /> 告警均有路由
                </Typography.Text>
              )}
            </CardFoot>
          </Card>
        </Col>
      </Row>

      <Card title="接入源健康" style={{ marginTop: 16 }}>
        <List<Source>
          loading={sources === null}
          dataSource={sources ?? []}
          locale={{ emptyText: <Empty description="还没有接入源" /> }}
          renderItem={(s) => (
            <List.Item
              extra={
                s.last_alert_at ? (
                  <Tooltip title={formatTime(s.last_alert_at)}>
                    <Typography.Text type="secondary">
                      最近告警 {dayjs(s.last_alert_at).fromNow()}
                    </Typography.Text>
                  </Tooltip>
                ) : (
                  <Typography.Text type="warning">从未收到告警</Typography.Text>
                )
              }
            >
              <List.Item.Meta
                title={
                  <>
                    {s.name}{' '}
                    {s.enabled ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>}
                  </>
                }
                description={s.description || undefined}
              />
            </List.Item>
          )}
        />
      </Card>
    </div>
  )
}
