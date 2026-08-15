import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Form,
  Input,
  List,
  Popconfirm,
  Row,
  Select,
  Space,
  Tag,
  Typography,
  message,
} from 'antd'
import { CopyOutlined, DeleteOutlined, EyeOutlined, PlusOutlined, SaveOutlined } from '@ant-design/icons'

import { createTemplate, deleteTemplate, listTemplates, previewTemplate, updateTemplate } from '../api'
import { errorMessage } from '../api/client'
import type { Template } from '../api/types'
import { useAuth } from '../auth/useAuth'

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

/** 剥离最基础的 lark_md 标记，做纯文本展示 */
function stripMd(s: string): string {
  return s.replace(/\*\*/g, '').replace(/<at[^>]*><\/at>/g, '@')
}

/** 本地模拟的飞书卡片预览：header 颜色/标题 + 文本、按钮、分栏等常见元素的近似渲染 */
function CardPreview({ rendered }: { rendered: string }) {
  let msg: FeishuMessage
  try {
    msg = JSON.parse(rendered) as FeishuMessage
  } catch {
    return <Typography.Text type="warning">渲染结果不是合法 JSON，无法生成卡片预览</Typography.Text>
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

export default function TemplatesPage() {
  const { canWrite } = useAuth()
  const [templates, setTemplates] = useState<Template[]>([])
  const [listLoading, setListLoading] = useState(false)
  const [editor, setEditor] = useState<EditorState | null>(null)
  const [saving, setSaving] = useState(false)
  const [previewing, setPreviewing] = useState(false)
  const [rendered, setRendered] = useState<string | null>(null)
  const [previewError, setPreviewError] = useState('')

  const load = useCallback(async (selectId?: number) => {
    setListLoading(true)
    try {
      const r = await listTemplates()
      setTemplates(r.list)
      if (selectId !== undefined) {
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
    setRendered(null)
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
    setRendered(null)
    setPreviewError('')
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
    setRendered(null)
    try {
      const r = await previewTemplate(editor.content)
      setRendered(r)
    } catch (err) {
      setPreviewError(errorMessage(err, '预览失败'))
    } finally {
      setPreviewing(false)
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

  return (
    <Row gutter={16}>
      <Col span={7}>
        <Card
          title="模板列表"
          size="small"
          extra={
            canWrite && (
              <Button
                size="small"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setEditor({ ...EMPTY_EDITOR })
                  setRendered(null)
                  setPreviewError('')
                }}
              >
                新建
              </Button>
            )
          }
        >
          <List
            loading={listLoading}
            dataSource={templates}
            renderItem={(t) => (
              <List.Item
                style={{
                  cursor: 'pointer',
                  background: editor?.id === t.id ? '#e6f4ff' : undefined,
                  padding: '8px 12px',
                  borderRadius: 4,
                }}
                onClick={() => selectTemplate(t)}
                actions={
                  canWrite
                    ? [
                        <Button
                          key="copy"
                          size="small"
                          type="text"
                          icon={<CopyOutlined />}
                          title="复制为新模板"
                          onClick={(e) => {
                            e.stopPropagation()
                            copyTemplate(t)
                          }}
                        />,
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
                                <Button
                                  size="small"
                                  type="text"
                                  danger
                                  icon={<DeleteOutlined />}
                                  onClick={(e) => e.stopPropagation()}
                                />
                              </Popconfirm>,
                            ]
                          : []),
                      ]
                    : undefined
                }
              >
                <List.Item.Meta
                  title={
                    <Space size={6}>
                      <span>{t.name}</span>
                      {t.is_builtin && <Tag color="gold">内置</Tag>}
                    </Space>
                  }
                  description={
                    <Typography.Text type="secondary" ellipsis style={{ fontSize: 12 }}>
                      {t.remark || t.channel_type}
                    </Typography.Text>
                  }
                />
              </List.Item>
            )}
          />
        </Card>
      </Col>

      <Col span={17}>
        {editor ? (
          <Card
            size="small"
            title={
              editor.id === null
                ? '新建模板'
                : editor.isBuiltin
                  ? `查看内置模板：${editor.name}`
                  : `编辑模板：${editor.name}`
            }
            extra={
              <Space>
                <Button icon={<EyeOutlined />} loading={previewing} onClick={() => void onPreview()}>
                  预览
                </Button>
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
                message="内置模板只读，可点击列表中的复制按钮创建可编辑副本。"
              />
            )}
            <Form layout="vertical" disabled={readOnly}>
              <Row gutter={12}>
                <Col span={10}>
                  <Form.Item label="模板名称" required style={{ marginBottom: 12 }}>
                    <Input
                      value={editor.name}
                      maxLength={64}
                      placeholder="如：critical-card"
                      onChange={(e) => setEditor({ ...editor, name: e.target.value })}
                    />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item label="渠道类型" required style={{ marginBottom: 12 }}>
                    <Select
                      value={editor.channelType}
                      options={[{ value: 'feishu', label: '飞书' }]}
                      onChange={(v) => setEditor({ ...editor, channelType: v })}
                    />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label="备注" style={{ marginBottom: 12 }}>
                    <Input
                      value={editor.remark}
                      maxLength={255}
                      placeholder="可选"
                      onChange={(e) => setEditor({ ...editor, remark: e.target.value })}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item
                label="模板内容（Go template 语法，渲染结果必须是合法飞书消息 JSON）"
                required
                style={{ marginBottom: 12 }}
                className="template-editor"
              >
                <Input.TextArea
                  value={editor.content}
                  rows={16}
                  spellCheck={false}
                  placeholder='{"msg_type":"text","content":{"text":"[{{ .Status }}] ..."}}'
                  onChange={(e) => setEditor({ ...editor, content: e.target.value })}
                />
              </Form.Item>
            </Form>

            {(previewError || rendered !== null) && (
              <>
                <Divider orientation="left" plain style={{ margin: '8px 0 12px' }}>
                  渲染预览
                </Divider>
                {previewError && (
                  <Alert type="error" showIcon message="渲染失败" description={previewError} />
                )}
                {rendered !== null && (
                  <Row gutter={16}>
                    <Col span={12}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        卡片效果
                      </Typography.Text>
                      <div style={{ marginTop: 8 }}>
                        <CardPreview rendered={rendered} />
                      </div>
                    </Col>
                    <Col span={12}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        渲染结果 JSON
                      </Typography.Text>
                      <pre className="json-view" style={{ marginTop: 8 }}>
                        {(() => {
                          try {
                            return JSON.stringify(JSON.parse(rendered), null, 2)
                          } catch {
                            return rendered
                          }
                        })()}
                      </pre>
                    </Col>
                  </Row>
                )}
              </>
            )}
          </Card>
        ) : (
          <Card size="small">
            <Typography.Text type="secondary">
              从左侧选择一个模板查看/编辑，或点击「新建」创建自定义模板。
            </Typography.Text>
          </Card>
        )}
      </Col>
    </Row>
  )
}
