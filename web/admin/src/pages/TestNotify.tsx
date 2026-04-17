import { useEffect, useMemo, useState } from 'react'
import { Table, Button, Tag, Modal, Typography, message, Popconfirm, Empty, Tooltip } from 'antd'
import { ReloadOutlined, DeleteOutlined, CopyOutlined } from '@ant-design/icons'
import { api } from '../api'

const { Text, Paragraph } = Typography

interface SinkRecord {
  id: number
  received_at: string
  method: string
  path: string
  slot: string
  remote_ip: string
  query: string
  headers: Record<string, string>
  body: string
  body_size: number
  truncated: boolean
}

const methodColor: Record<string, string> = {
  GET: 'blue',
  POST: 'green',
  PUT: 'orange',
  PATCH: 'purple',
  DELETE: 'red',
}

function formatTime(v: string) {
  return v ? v.slice(0, 19).replace('T', ' ') : ''
}

function tryPrettyJson(body: string) {
  const s = body.trim()
  if (!s.startsWith('{') && !s.startsWith('[')) return body
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return body
  }
}

export default function TestNotify() {
  const [list, setList] = useState<SinkRecord[]>([])
  const [capacity, setCapacity] = useState(0)
  const [loading, setLoading] = useState(false)
  const [detail, setDetail] = useState<SinkRecord | null>(null)

  const load = async () => {
    setLoading(true)
    try {
      const { data } = await api.get('/admin/test_notify')
      setList(data.data.list || [])
      setCapacity(data.data.capacity || 0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    const id = setInterval(load, 5000)
    return () => clearInterval(id)
  }, [])

  const clear = async () => {
    await api.delete('/admin/test_notify')
    message.success('已清空')
    load()
  }

  const sinkBase = useMemo(() => {
    const { protocol, host } = window.location
    return `${protocol}//${host}/test/notify`
  }, [])

  const copy = (v: string) => {
    navigator.clipboard.writeText(v)
    message.success('已复制')
  }

  return (
    <>
      <div style={{
        background: 'var(--bg-surface)',
        border: '1px solid var(--border-hairline)',
        padding: '14px 20px',
        marginBottom: 14,
        position: 'relative',
      }}>
        <div style={{
          position: 'absolute', top: 0, left: 0,
          width: 28, height: 1, background: 'var(--accent-gold)', opacity: 0.6,
        }} />
        <div style={{
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
          marginBottom: 6,
        }}>
          <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
            回调接收地址 · 可用作 notify_url
          </span>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button size="small" icon={<ReloadOutlined />} onClick={load} loading={loading}>刷新</Button>
            <Popconfirm title="清空所有回调记录？" onConfirm={clear} okText="清空" cancelText="取消">
              <Button size="small" danger icon={<DeleteOutlined />}>清空</Button>
            </Popconfirm>
          </div>
        </div>
        <div style={{
          display: 'flex', alignItems: 'center', gap: 10,
          fontFamily: 'var(--font-mono)', fontSize: 14,
          wordBreak: 'break-all',
        }}>
          <span style={{ color: 'var(--accent-gold)' }}>{sinkBase}</span>
          <span style={{ color: 'var(--text-tertiary)' }}>/{'{任意后缀}'}</span>
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            style={{ color: 'var(--text-tertiary)' }}
            onClick={() => copy(sinkBase + '/demo')}
          />
        </div>
        <div style={{
          marginTop: 6, fontSize: 11, color: 'var(--text-tertiary)',
        }}>
          ● 内存 ring buffer · 容量 {capacity} 条 · 重启丢失
        </div>
      </div>

      <Table
        rowKey="id"
        size="small"
        dataSource={list}
        loading={loading}
        pagination={false}
        scroll={{ x: 'max-content', y: '100%' }}
        locale={{ emptyText: <Empty description="暂无回调记录，向上方地址发一个 POST 试试" /> }}
        onRow={(row) => ({ onClick: () => setDetail(row), style: { cursor: 'pointer' } })}
        columns={[
          {
            title: '时间',
            dataIndex: 'received_at',
            width: 200,
            render: (v: string) => (
              <Tooltip title={v}>
                <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{formatTime(v)}</span>
              </Tooltip>
            ),
          },
          {
            title: '方法',
            dataIndex: 'method',
            width: 90,
            render: (v: string) => <Tag color={methodColor[v] || 'default'}>{v}</Tag>,
          },
          {
            title: '路径',
            dataIndex: 'path',
            width: 340,
            ellipsis: true,
            render: (v: string) => (
              <Tooltip title={v}>
                <span className="mono" style={{ fontSize: 12, color: 'var(--text-primary)', whiteSpace: 'nowrap' }}>{v}</span>
              </Tooltip>
            ),
          },
          {
            title: '来源',
            dataIndex: 'remote_ip',
            width: 130,
            render: (v: string) => <span className="mono" style={{ fontSize: 11, color: 'var(--text-tertiary)' }}>{v}</span>,
          },
          {
            title: '大小',
            dataIndex: 'body_size',
            width: 90,
            render: (v: number, row) => (
              <span className="mono" style={{ fontSize: 11 }}>
                {v} B{row.truncated ? <span style={{ color: 'var(--accent-crimson)' }}> +</span> : null}
              </span>
            ),
          },
          {
            title: 'Body 预览',
            dataIndex: 'body',
            ellipsis: true,
            render: (v: string) => v
              ? (
                <Tooltip title={<pre style={{ margin: 0, maxHeight: 240, overflow: 'auto', whiteSpace: 'pre-wrap', wordBreak: 'break-all', fontSize: 11 }}>{tryPrettyJson(v).slice(0, 800)}</pre>} overlayStyle={{ maxWidth: 520 }}>
                  <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{v.replace(/\s+/g, ' ')}</span>
                </Tooltip>
              )
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
        ]}
      />

      <Modal
        open={!!detail}
        onCancel={() => setDetail(null)}
        footer={<Button onClick={() => setDetail(null)}>关闭</Button>}
        title={detail ? `#${detail.id} · ${detail.method} ${detail.path}` : ''}
        width={780}
      >
        {detail && (
          <div>
            <Paragraph style={{ marginBottom: 6 }}>
              <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>接收时间</Text>
              <br />
              <Text className="mono">{formatTime(detail.received_at)}</Text>
            </Paragraph>
            {detail.query && (
              <Paragraph style={{ marginBottom: 6 }}>
                <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>Query</Text>
                <br />
                <Text code copyable>{detail.query}</Text>
              </Paragraph>
            )}
            <Paragraph style={{ marginBottom: 6 }}>
              <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>Headers</Text>
            </Paragraph>
            <pre style={{
              background: 'var(--bg-deep)',
              border: '1px solid var(--border-hairline)',
              padding: 12,
              fontSize: 11,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-secondary)',
              maxHeight: 180,
              overflow: 'auto',
              marginBottom: 12,
            }}>
              {Object.entries(detail.headers).map(([k, v]) => `${k}: ${v}`).join('\n')}
            </pre>
            <Paragraph style={{ marginBottom: 6 }}>
              <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>
                Body {detail.truncated ? <span style={{ color: 'var(--accent-crimson)' }}>· 已截断</span> : null}
              </Text>
            </Paragraph>
            <pre style={{
              background: 'var(--bg-deep)',
              border: '1px solid var(--border-hairline)',
              padding: 12,
              fontSize: 11,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-primary)',
              maxHeight: 300,
              overflow: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}>
              {tryPrettyJson(detail.body) || '(empty)'}
            </pre>
          </div>
        )}
      </Modal>
    </>
  )
}
