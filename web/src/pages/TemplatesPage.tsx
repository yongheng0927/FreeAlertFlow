import { useCallback, useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
  Divider,
  Form,
  Input,
  Modal,
  Popconfirm,
  Radio,
  Row,
  Select,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import {
  ArrowLeftOutlined,
  CopyOutlined,
  DeleteOutlined,
  EyeOutlined,
  PlusOutlined,
  SaveOutlined,
  SearchOutlined,
  SendOutlined,
} from '@ant-design/icons'

import {
  createTemplate,
  deleteTemplate,
  listChannels,
  listTemplates,
  previewTemplate,
  testTemplateSend,
  updateTemplate,
} from '../api'
import { errorMessage } from '../api/client'
import type { Channel, Template, TemplatePreview, TestSendResult } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { formatTime } from '../utils'

const HEADER_COLORS: Record<string, string> = {
  red: '#f5222d',
  orange: '#fa8c16',
  green: '#52c41a',
  blue: '#1677ff',
  yellow: '#fadb14',
  grey: '#8c8c8c',
  purple: '#722ed1',
  indigo: '#2f54eb',
  turquoise: '#13c2c2',
  violet: '#eb2f96',
  carmine: '#cf1322',
  wathet: '#69c0ff',
}

interface FeishuText {
  tag?: string
  content?: string
}

interface FeishuElement {
  tag?: string
  text?: FeishuText
  fields?: { text?: FeishuText }[]
  actions?: { text?: FeishuText; url?: string; type?: string }[]
}

interface FeishuCard {
  header?: { template?: string; title?: FeishuText }
  elements?: FeishuElement[]
}

interface FeishuMessage {
  msg_type?: string
  card?: FeishuCard
  content?: { text?: string }
}

/** 剥离最基础的 markdown 标记，做纯文本展示 */
function stripMd(s: string): string {
  return s
    .replace(/\*\*/g, '')
    .replace(/~~/g, '')
    .replace(/<at[^>]*><\/at>/g, '@')
}

/** 格式化 JSON 字符串，失败时保留原文 */
function prettyJson(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

/** 本地模拟的飞书卡片预览：header 颜色/标题 + 文本、按钮、分栏等常见元素的近似渲染 */
function CardPreview({ payload }: { payload: string }) {
  let msg: FeishuMessage
  try {
    msg = JSON.parse(payload) as FeishuMessage
  } catch {
    return <Typography.Text type="warning">飞书消息体不是合法 JSON，无法生成卡片预览</Typography.Text>
  }

  if (msg.msg_type === 'text') {
    return (
      <Card size="small" style={{ maxWidth: 480 }}>
        <div style={{ whiteSpace: 'pre-wrap' }}>{stripMd(msg.content?.text ?? '')}</div>
      </Card>
    )
  }

  if (msg.msg_type !== 'interactive' || !msg.card) {
    return <Typography.Text type="secondary">暂不支持预览 msg_type={msg.msg_type ?? '未知'} 的卡片效果</Typography.Text>
  }

  const header = msg.card.header
  const headerColor = HEADER_COLORS[header?.template ?? ''] ?? '#1677ff'

  return (
    <Card
      size="small"
      style={{ maxWidth: 480, borderTop: `4px solid ${headerColor}` }}
      title={
        header?.title?.content ? (
          <span style={{ color: headerColor }}>{stripMd(header.title.content)}</span>
        ) : undefined
      }
    >
      {(msg.card.elements ?? []).map((el, i) => {
        if (el.tag === 'hr') return <Divider key={i} style={{ margin: '8px 0' }} />
        if (el.tag === 'action') {
          return (
            <Space key={i} wrap>
              {(el.actions ?? []).map((a, j) => (
                <Button key={j} size="small" type={a.type === 'primary' ? 'primary' : 'default'} href={a.url} target="_blank">
                  {a.text?.content ?? '按钮'}
                </Button>
              ))}
            </Space>
          )
        }
        if (el.fields && el.fields.length > 0) {
          return (
            <Row key={i} gutter={[8, 8]} style={{ marginBottom: 8 }}>
              {el.fields.map((f, j) => (
                <Col span={12} key={j} style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>
                  {stripMd(f.text?.content ?? '')}
                </Col>
              ))}
            </Row>
          )
        }
        return (
          <div key={i} style={{ whiteSpace: 'pre-wrap', marginBottom: 8, fontSize: 12 }}>
            {stripMd(el.text?.content ?? '')}
          </div>
        )
      })}
    </Card>
  )
}

/** 各渠道类型的 payload 结构说明与小示例 */
const CHANNEL_PAYLOAD_HELP: Record<string, { desc: ReactNode; example: string }> = {
  feishu: {
    desc: (
      <>
        飞书机器人消息体：合法 JSON 且 <code>msg_type</code> 为 <code>text</code> / <code>post</code> /{' '}
        <code>interactive</code> / <code>image</code> / <code>share_chat</code> 之一。
        卡片消息用 <code>msg_type=interactive</code>：<code>card.header.template</code> 是头部颜色
        （red/orange/green/blue 等），<code>card.header.title</code> 是标题，<code>card.elements</code>{' '}
        是内容元素（div + lark_md 文本、hr 分割线、action 按钮等）。
      </>
    ),
    example: `{
  "msg_type": "interactive",
  "card": {
    "header": {
      "template": "{{ severityColor (label .CommonLabels "severity") }}",
      "title": {"tag": "plain_text", "content": "[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}"}
    },
    "elements": [
      {"tag": "div", "text": {"tag": "lark_md", "content": "**来源** {{ jesc .SourceName }}"}}
    ]
  }
}`,
  },
  dingtalk: {
    desc: (
      <>
        钉钉机器人消息体：合法 JSON 且 <code>msgtype</code> 为 <code>text</code> 或 <code>markdown</code>。
        markdown 消息的正文在 <code>markdown.text</code>，通知标题在 <code>markdown.title</code>；
        钉钉 markdown 支持标题、加粗、链接、列表等语法。勾选了「@所有人」的渠道会由发送层自动注入{' '}
        <code>at.isAtAll</code>，模板无需关心。
      </>
    ),
    example: `{
  "msgtype": "markdown",
  "markdown": {
    "title": "[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}",
    "text": "## 告警\\n\\n- **来源**：{{ jesc .SourceName }}"
  }
}`,
  },
  wecom: {
    desc: (
      <>
        企业微信机器人消息体：合法 JSON 且 <code>msgtype</code> 为 <code>text</code> 或 <code>markdown</code>。
        markdown 消息的正文在 <code>markdown.content</code>；企微 markdown 是子集：支持标题、加粗、
        链接、行内代码、引用，<strong>不支持列表、图片和 @人</strong>。
      </>
    ),
    example: `{
  "msgtype": "markdown",
  "markdown": {
    "content": "## [{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}\\n> 来源：{{ jesc .SourceName }}"
  }
}`,
  },
}

/** 按当前渠道类型生成语法帮助面板 */
function buildSyntaxHelp(channelType: string) {
  const ch = CHANNEL_PAYLOAD_HELP[channelType] ?? CHANNEL_PAYLOAD_HELP.feishu
  return [
    {
      key: 'payload',
      label: '消息 payload 结构（按渠道类型）',
      children: (
        <Typography style={{ fontSize: 12 }}>
          <p style={{ marginBottom: 8 }}>{ch.desc}</p>
          <p style={{ marginBottom: 4 }}>示例：</p>
          <pre className="json-view" style={{ marginTop: 0 }}>
            {ch.example}
          </pre>
          <Typography.Text type="secondary">
            模板渲染结果就是直接发送给机器人的消息体；保存和预览时会按渠道类型校验 JSON 合法性。
            把插值嵌入 JSON 字符串时务必用 <code>| jesc</code> 转义。
          </Typography.Text>
        </Typography>
      ),
    },
    {
      key: 'template',
      label: '插值变量与函数（Go text/template）',
      children: (
        <Typography style={{ fontSize: 12 }}>
          <p style={{ marginBottom: 4 }}>可用变量：</p>
          <ul style={{ paddingLeft: 18, marginBottom: 8 }}>
            <li>
              <code>{'{{ .Status }}'}</code>、<code>{'{{ .Receiver }}'}</code>、
              <code>{'{{ .GroupKey }}'}</code>、<code>{'{{ .Version }}'}</code>
            </li>
            <li>
              <code>{'{{ .ExternalURL }}'}</code>、<code>{'{{ .RootURL }}'}</code>、
              <code>{'{{ .SourceName }}'}</code>
            </li>
            <li>
              <code>{'{{ .GroupLabels }}'}</code>、<code>{'{{ .CommonLabels }}'}</code>、
              <code>{'{{ .CommonAnnotations }}'}</code>（map）
            </li>
            <li>
              <code>{'{{ range .Alerts }}'}</code> 遍历告警，字段：
              <code>.Status</code>、<code>.Labels</code>、<code>.Annotations</code>、
              <code>.StartsAt</code>、<code>.EndsAt</code>、<code>.GeneratorURL</code>、
              <code>.Fingerprint</code>
            </li>
          </ul>
          <p style={{ marginBottom: 4 }}>常用函数：</p>
          <ul style={{ paddingLeft: 18, marginBottom: 8 }}>
            <li>
              <code>{'{{ label .CommonLabels "severity" }}'}</code> 取标签值
            </li>
            <li>
              <code>{'{{ timeFormat "2006-01-02 15:04:05" .StartsAt }}'}</code> 格式化时间
            </li>
            <li>
              <code>{'{{ jesc .SourceName }}'}</code> 转义后可安全嵌入 JSON 字符串
            </li>
            <li>
              <code>{'{{ mdEscape .CommonAnnotations.summary }}'}</code> 转义 markdown 元字符
            </li>
            <li>
              <code>{'{{ severityColor "critical" }}'}</code> severity 映射为卡片颜色；{' '}
              <code>truncate</code> 截断文本
            </li>
            <li>同时支持 Sprig 函数库（如 <code>upper</code>）与 Go template 管道语法</li>
          </ul>
        </Typography>
      ),
    },
  ]
}

interface EditorState {
  /** null = 新建 */
  id: number | null
  isBuiltin: boolean
  name: string
  channelType: string
  remark: string
  content: string
}

const EMPTY_EDITOR: EditorState = {
  id: null,
  isBuiltin: false,
  name: '',
  channelType: 'feishu',
  remark: '',
  content: '',
}

/** 模板渠道类型元数据，颜色与渠道页一致 */
const TEMPLATE_CHANNEL_TYPES: Record<string, { label: string; color: string }> = {
  feishu: { label: '飞书', color: 'blue' },
  dingtalk: { label: '钉钉', color: 'cyan' },
  wecom: { label: '企业微信', color: 'green' },
}

function TemplateChannelTypeTag({ type }: { type: string }) {
  const meta = TEMPLATE_CHANNEL_TYPES[type]
  return meta ? <Tag color={meta.color}>{meta.label}</Tag> : <Tag>{type || '-'}</Tag>
}

type SortBy = 'updated' | 'created'

/** 排序偏好持久化 key */
const SORT_KEY = 'fenghuo.templates.sort'

function loadSortBy(): SortBy {
  return localStorage.getItem(SORT_KEY) === 'created' ? 'created' : 'updated'
}

export default function TemplatesPage() {
  const { canWrite } = useAuth()
  const [templates, setTemplates] = useState<Template[]>([])
  const [listLoading, setListLoading] = useState(false)
  const [typeFilter, setTypeFilter] = useState('')
  const [keyword, setKeyword] = useState('')
  const [sortBy, setSortByState] = useState<SortBy>(loadSortBy)
  const [recentId, setRecentId] = useState<number | null>(null)
  const [editor, setEditor] = useState<EditorState | null>(null)
  // 编辑器内容被修改后置位，返回列表前据此提示未保存
  const [dirty, setDirty] = useState(false)
  const [saving, setSaving] = useState(false)
  const [previewing, setPreviewing] = useState(false)
  const [preview, setPreview] = useState<TemplatePreview | null>(null)
  const [previewError, setPreviewError] = useState('')
  const [testOpen, setTestOpen] = useState(false)
  const [testChannels, setTestChannels] = useState<Channel[]>([])
  const [testChannelId, setTestChannelId] = useState<number | undefined>(undefined)
  const [testSending, setTestSending] = useState(false)
  const [testResult, setTestResult] = useState<TestSendResult | null>(null)

  const setSortBy = (v: SortBy) => {
    setSortByState(v)
    localStorage.setItem(SORT_KEY, v)
  }

  const load = useCallback(async (selectId?: number) => {
    setListLoading(true)
    try {
      const r = await listTemplates()
      setTemplates(r.list)
      if (selectId !== undefined) {
        // 新建/保存后落位高亮几秒，告诉用户卡片在哪
        setRecentId(selectId)
        window.setTimeout(() => setRecentId(null), 3000)
        const t = r.list.find((x) => x.id === selectId)
        if (t) selectTemplate(t)
      }
    } catch (err) {
      message.error(errorMessage(err, '加载模板列表失败'))
    } finally {
      setListLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const selectTemplate = (t: Template) => {
    setEditor({
      id: t.id,
      isBuiltin: t.is_builtin,
      name: t.name,
      channelType: t.channel_type,
      remark: t.remark,
      content: t.content,
    })
    setDirty(false)
    setPreview(null)
    setPreviewError('')
  }

  const openNew = () => {
    setEditor({ ...EMPTY_EDITOR })
    setDirty(false)
    setPreview(null)
    setPreviewError('')
  }

  const copyTemplate = (t: Template) => {
    setEditor({
      id: null,
      isBuiltin: false,
      name: `${t.name}-copy`,
      channelType: t.channel_type,
      remark: t.remark,
      content: t.content,
    })
    setDirty(false)
    setPreview(null)
    setPreviewError('')
  }

  // 编辑器字段更新：同步置位 dirty
  const updateEditor = (patch: Partial<EditorState>) => {
    setEditor((e) => (e === null ? e : { ...e, ...patch }))
    setDirty(true)
  }

  // 返回列表：有未保存修改时二次确认
  const closeEditor = () => {
    if (!dirty) {
      setEditor(null)
      return
    }
    Modal.confirm({
      title: '有未保存的修改，确定要离开吗？',
      okText: '离开',
      cancelText: '取消',
      onOk: () => {
        setEditor(null)
        setDirty(false)
      },
    })
  }

  const onDelete = async (t: Template) => {
    try {
      await deleteTemplate(t.id)
      message.success('已删除')
      if (editor?.id === t.id) setEditor(null)
      void load()
    } catch (err) {
      message.error(errorMessage(err, '删除失败'), 6)
    }
  }

  const onPreview = async () => {
    if (!editor) return
    setPreviewing(true)
    setPreviewError('')
    setPreview(null)
    try {
      const r = await previewTemplate(editor.content, editor.channelType)
      setPreview(r)
    } catch (err) {
      setPreviewError(errorMessage(err, '预览失败'))
    } finally {
      setPreviewing(false)
    }
  }

  const openTestSend = async () => {
    if (!editor) return
    setTestResult(null)
    setTestChannelId(undefined)
    try {
      const r = await listChannels()
      const usable = r.list.filter((c) => c.type === editor.channelType)
      setTestChannels(usable)
      if (usable.length === 1) setTestChannelId(usable[0].id)
      setTestOpen(true)
    } catch (err) {
      message.error(errorMessage(err, '加载渠道列表失败'), 6)
    }
  }

  const onTestSend = async () => {
    if (!editor || testChannelId === undefined) return
    setTestSending(true)
    setTestResult(null)
    try {
      const r = await testTemplateSend(editor.content, editor.channelType, testChannelId)
      setTestResult(r)
    } catch (err) {
      message.error(errorMessage(err, '发送测试失败'), 6)
    } finally {
      setTestSending(false)
    }
  }

  const onSave = async () => {
    if (!editor) return
    setSaving(true)
    try {
      const input = {
        name: editor.name.trim(),
        channel_type: editor.channelType,
        content: editor.content,
        remark: editor.remark,
      }
      if (editor.id === null) {
        const created = await createTemplate(input)
        message.success('模板已创建')
        void load(created.id)
      } else {
        const updated = await updateTemplate(editor.id, input)
        message.success('已保存')
        void load(updated.id)
      }
    } catch (err) {
      // 后端保存时会做渲染校验，400 里带具体原因
      message.error(errorMessage(err, '保存失败'), 8)
    } finally {
      setSaving(false)
    }
  }

  const readOnly = !canWrite || (editor?.isBuiltin ?? false)

  // 渠道过滤 + 关键词搜索（名称/备注）+ 排序：同排序值时自定义模板排在内置之前
  const kw = keyword.trim().toLowerCase()
  const shown = [...templates]
    .filter((t) => !typeFilter || t.channel_type === typeFilter)
    .filter(
      (t) =>
        !kw ||
        t.name.toLowerCase().includes(kw) ||
        (t.remark ?? '').toLowerCase().includes(kw),
    )
    .sort((a, b) => {
      const diff =
        sortBy === 'updated' ? b.updated_at.localeCompare(a.updated_at) : b.id - a.id
      if (diff !== 0) return diff
      return Number(a.is_builtin) - Number(b.is_builtin) || b.id - a.id
    })

  const editorTitle = editor
    ? editor.id === null
      ? '新建模板'
      : editor.isBuiltin
        ? `查看内置模板：${editor.name}`
        : `编辑模板：${editor.name}`
    : ''

  // 编辑态：整页容器（与路由规则页同模式），左编辑右预览
  if (editor !== null) {
    return (
      <Card
        size="small"
        title={
          <Space size={4}>
            <Button
              type="text"
              size="small"
              icon={<ArrowLeftOutlined />}
              onClick={closeEditor}
            />
            <span>{editorTitle}</span>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<EyeOutlined />} loading={previewing} onClick={() => void onPreview()}>
              预览
            </Button>
            {canWrite && (
              <Button icon={<SendOutlined />} onClick={() => void openTestSend()}>
                发送测试
              </Button>
            )}
            {!readOnly && (
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={saving}
                disabled={!editor.name.trim() || !editor.content.trim()}
                onClick={() => void onSave()}
              >
                保存
              </Button>
            )}
          </Space>
        }
      >
        {editor.isBuiltin && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="内置模板只读，可点击卡片上的复制按钮创建可编辑副本。"
          />
        )}
        <Row gutter={16}>
          <Col xl={14}>
            <Form layout="vertical" disabled={readOnly}>
              <Row gutter={12}>
                <Col span={8}>
                  <Form.Item label="模板名称" required style={{ marginBottom: 12 }}>
                    <Input
                      value={editor.name}
                      maxLength={64}
                      placeholder="如：critical-alert"
                      onChange={(e) => updateEditor({ name: e.target.value })}
                    />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item label="渠道类型" required style={{ marginBottom: 12 }}>
                    <Select
                      value={editor.channelType}
                      options={Object.entries(TEMPLATE_CHANNEL_TYPES).map(([value, m]) => ({
                        value,
                        label: m.label,
                      }))}
                      onChange={(v) => updateEditor({ channelType: v })}
                    />
                  </Form.Item>
                </Col>
                <Col span={10}>
                  <Form.Item label="备注" style={{ marginBottom: 12 }}>
                    <Input
                      value={editor.remark}
                      maxLength={255}
                      placeholder="一句话说明这个模板是做什么的，会显示在卡片上"
                      onChange={(e) => updateEditor({ remark: e.target.value })}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item
                label="模板内容（所选渠道消息 payload 的 Go template，渲染结果直接发送）"
                required
                style={{ marginBottom: 12 }}
                className="template-editor"
              >
                <Input.TextArea
                  value={editor.content}
                  rows={22}
                  spellCheck={false}
                  placeholder={
                    '{"msg_type":"text","content":{"text":"[{{ .Status | upper }}] {{ label .CommonLabels "alertname" | jesc }}"}}'
                  }
                  onChange={(e) => updateEditor({ content: e.target.value })}
                />
              </Form.Item>
            </Form>

            <Collapse size="small" ghost items={buildSyntaxHelp(editor.channelType)} style={{ marginBottom: 12 }} />
          </Col>

          <Col xl={10}>
            <Divider orientation="left" plain style={{ margin: '0 0 12px' }}>
              渲染预览
            </Divider>
            {previewError === '' && preview === null && (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                点击右上角「预览」，用样例告警渲染当前模板；点「发送测试」可真实投递到渠道，在
                IM 客户端查看最终效果。
              </Typography.Text>
            )}
            {previewError && (
              <Alert type="error" showIcon message="渲染失败" description={previewError} />
            )}
            {preview !== null && (
              <>
                {editor.channelType === 'feishu' && (
                  <>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      卡片效果（本地模拟，以实际发送为准）
                    </Typography.Text>
                    <div style={{ marginTop: 8, marginBottom: 16 }}>
                      <CardPreview payload={preview.rendered} />
                    </div>
                  </>
                )}
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  消息体 JSON
                  {editor.channelType !== 'feishu' && '（以实际发送为准）'}
                </Typography.Text>
                <pre className="json-view" style={{ marginTop: 8 }}>
                  {prettyJson(preview.rendered)}
                </pre>
              </>
            )}
          </Col>
        </Row>

        <Modal
          title="发送测试"
          open={testOpen}
          okText="发送"
          cancelText="关闭"
          okButtonProps={{ disabled: testChannelId === undefined, loading: testSending }}
          cancelButtonProps={{ disabled: testSending }}
          maskClosable={!testSending}
          onOk={() => void onTestSend()}
          onCancel={() => {
            if (testSending) return
            setTestOpen(false)
          }}
        >
          <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
            用当前编辑器中的模板内容渲染样例告警，真实投递到所选渠道（不会写入投递记录）。
          </Typography.Paragraph>
          {testChannels.length === 0 ? (
            <Alert
              type="warning"
              showIcon
              message={`没有 ${TEMPLATE_CHANNEL_TYPES[editor.channelType]?.label ?? editor.channelType} 类型的渠道，请先到「通知渠道」页创建。`}
            />
          ) : (
            <Select
              style={{ width: '100%' }}
              placeholder="选择接收测试消息的渠道"
              value={testChannelId}
              onChange={(v) => setTestChannelId(v)}
              options={testChannels.map((c) => ({ value: c.id, label: c.name }))}
            />
          )}
          {testResult && (
            <Alert
              style={{ marginTop: 12 }}
              type={testResult.success ? 'success' : 'error'}
              showIcon
              message={testResult.success ? `发送成功，耗时 ${testResult.duration_ms} ms` : '发送失败'}
              description={
                testResult.success
                  ? undefined
                  : `HTTP ${testResult.http_status}，code=${testResult.code}，${testResult.msg || '-'}`
              }
            />
          )}
        </Modal>
      </Card>
    )
  }

  // 列表态：筛选/排序工具栏 + 卡片墙
  return (
    <>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 12 }} wrap>
        <Radio.Group
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value as string)}
          optionType="button"
          buttonStyle="solid"
          options={[
            { label: '全部', value: '' },
            ...Object.entries(TEMPLATE_CHANNEL_TYPES).map(([value, m]) => ({
              label: m.label,
              value,
            })),
          ]}
        />
        <Input
          allowClear
          prefix={<SearchOutlined style={{ color: '#bfbfbf' }} />}
          placeholder="搜索名称 / 备注"
          style={{ width: 220 }}
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
        />
        <Radio.Group
          value={sortBy}
          onChange={(e) => setSortBy(e.target.value as SortBy)}
          optionType="button"
          buttonStyle="solid"
          options={[
            { label: '最近更新', value: 'updated' },
            { label: '最新创建', value: 'created' },
          ]}
        />
      </Space>

      <Spin spinning={listLoading}>
        <Row gutter={[12, 12]}>
          {canWrite && (
            <Col xs={24} sm={12} lg={8} xl={6}>
              <Card
                size="small"
                hoverable
                style={{ borderStyle: 'dashed' }}
                styles={{
                  body: { height: 96, display: 'flex', alignItems: 'center', justifyContent: 'center' },
                }}
                onClick={openNew}
              >
                <Button type="link" icon={<PlusOutlined />}>
                  新建模板
                </Button>
              </Card>
            </Col>
          )}
          {shown.map((t) => (
            <Col xs={24} sm={12} lg={8} xl={6} key={t.id}>
              <Card
                size="small"
                hoverable
                onClick={() => selectTemplate(t)}
                style={t.id === recentId ? { boxShadow: '0 0 0 2px #1677ff' } : undefined}
                styles={{ body: { height: 96 } }}
                actions={
                  canWrite
                    ? [
                        <Tooltip title="复制为新模板" key="copy">
                          <CopyOutlined
                            onClick={(e) => {
                              e.stopPropagation()
                              copyTemplate(t)
                            }}
                          />
                        </Tooltip>,
                        ...(!t.is_builtin
                          ? [
                              <Popconfirm
                                key="del"
                                title="删除模板"
                                description="被渠道绑定的模板无法删除。确认删除？"
                                okText="删除"
                                okButtonProps={{ danger: true }}
                                cancelText="取消"
                                onConfirm={() => void onDelete(t)}
                              >
                                <span onClick={(e) => e.stopPropagation()}>
                                  <DeleteOutlined />
                                </span>
                              </Popconfirm>,
                            ]
                          : []),
                      ]
                    : undefined
                }
              >
                <Space size={4} style={{ width: '100%', justifyContent: 'space-between' }}>
                  <Typography.Text strong ellipsis={{ tooltip: t.name }} style={{ maxWidth: 150 }}>
                    {t.name}
                  </Typography.Text>
                  <span style={{ whiteSpace: 'nowrap' }}>
                    <TemplateChannelTypeTag type={t.channel_type} />
                    {t.is_builtin && <Tag color="gold">内置</Tag>}
                  </span>
                </Space>
                <Typography.Paragraph
                  type="secondary"
                  style={{ fontSize: 12, marginTop: 6, marginBottom: 4 }}
                  ellipsis={{ rows: 1, tooltip: t.remark || '无备注' }}
                >
                  {t.remark || '无备注'}
                </Typography.Paragraph>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  更新于 {formatTime(t.updated_at)}
                </Typography.Text>
              </Card>
            </Col>
          ))}
        </Row>
        {!listLoading && shown.length === 0 && (
          <Typography.Text type="secondary">
            {kw ? '没有匹配的模板，换个关键词试试。' : '该渠道类型下暂无模板。'}
          </Typography.Text>
        )}
      </Spin>
    </>
  )
}
