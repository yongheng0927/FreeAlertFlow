import type { CSSProperties, ReactNode } from 'react'
import { Button } from 'antd'
import { CopyOutlined } from '@ant-design/icons'

import { copyText } from '../utils'

// 配色对齐常见 JSON 高亮（GitHub 风格），与 .json-view 的浅色底搭配
const COLOR_KEY = '#0550ae'
const COLOR_STRING = '#0a7d32'
const COLOR_NUMBER = '#953800'
const COLOR_BOOL = '#cf222e'
const COLOR_NULL = '#6e7781'

const containerStyle: CSSProperties = {
  background: '#f6f8fa',
  border: '1px solid #e8e8e8',
  borderRadius: 6,
  padding: 12,
  maxHeight: 480,
  overflow: 'auto',
  fontFamily: "'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace",
  fontSize: 12,
  lineHeight: 1.7,
}

function Primitive({ value }: { value: unknown }) {
  if (value === null) return <span style={{ color: COLOR_NULL }}>null</span>
  switch (typeof value) {
    case 'string':
      return <span style={{ color: COLOR_STRING, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>&quot;{value}&quot;</span>
    case 'number':
      return <span style={{ color: COLOR_NUMBER }}>{String(value)}</span>
    case 'boolean':
      return <span style={{ color: COLOR_BOOL }}>{String(value)}</span>
    default:
      return <span>{String(value)}</span>
  }
}

function Node({ name, value }: { name: ReactNode; value: unknown }) {
  // 键名后的 ": " 分隔；数组元素没有键名，仅渲染值
  const label = name ? (
    <>
      {name}
      <span style={{ color: COLOR_NULL }}>: </span>
    </>
  ) : null

  const isObj = typeof value === 'object' && value !== null
  if (!isObj) {
    return (
      <div>
        {label}
        <Primitive value={value} />
      </div>
    )
  }

  const entries = Array.isArray(value)
    ? value.map((v, i) => [String(i), v] as const)
    : Object.entries(value as Record<string, unknown>)
  const openMark = Array.isArray(value) ? '[' : '{'
  const closeMark = Array.isArray(value) ? ']' : '}'

  if (entries.length === 0) {
    return (
      <div>
        {label}
        <span style={{ color: COLOR_NULL }}>
          {openMark}
          {closeMark}
        </span>
      </div>
    )
  }

  return (
    <div>
      <div>
        {label}
        <span style={{ color: COLOR_NULL }}>{openMark}</span>
      </div>
      <div style={{ paddingLeft: 18, borderLeft: '1px solid #e8e8e8', marginLeft: 4 }}>
        {entries.map(([k, v]) => (
          <Node
            key={k}
            name={
              Array.isArray(value) ? null : (
                <span style={{ color: COLOR_KEY }}>&quot;{k}&quot;</span>
              )
            }
            value={v}
          />
        ))}
      </div>
      <div>
        <span style={{ color: COLOR_NULL }}>{closeMark}</span>
      </div>
    </div>
  )
}

/** JsonView 以带语法高亮的树形结构展示 JSON 数据（告警原始 payload 等） */
export function JsonView({ data }: { data: unknown }) {
  return (
    <div style={{ position: 'relative' }}>
      <Button
        size="small"
        type="text"
        icon={<CopyOutlined />}
        style={{ position: 'absolute', top: 4, right: 4, zIndex: 1 }}
        onClick={() => void copyText(JSON.stringify(data, null, 2))}
      />
      <div style={containerStyle}>
        <Node name={null} value={data} />
      </div>
    </div>
  )
}
