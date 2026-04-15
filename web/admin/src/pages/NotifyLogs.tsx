import { useState } from 'react'
import { Table, Tag, Input, Button, message, Tooltip } from 'antd'
import { SearchOutlined, RedoOutlined } from '@ant-design/icons'
import { api } from '../api'

const statusColor: Record<string, string> = {
  pending: 'orange',
  success: 'green',
  failed: 'red',
  dropped: 'default',
}

const statusLabel: Record<string, string> = {
  pending: '待投递',
  success: '已成功',
  failed: '已失败',
  dropped: '已丢弃',
}

export default function NotifyLogs() {
  const [orderNo, setOrderNo] = useState('')
  const [list, setList] = useState<any[]>([])

  const load = async () => {
    if (!orderNo) {
      message.warning('请输入平台订单号')
      return
    }
    const { data } = await api.get('/admin/notify_logs', { params: { order_no: orderNo } })
    setList(data.data)
  }

  const retry = async (id: number) => {
    await api.post(`/admin/notify_logs/${id}/retry`)
    message.success('已标记重试')
    load()
  }

  return (
    <>
      <div className="ep-page-header">
        <div className="col-title">
          <div className="eyebrow">监控 · 03</div>
          <h1>通知<em>日志</em></h1>
        </div>
      </div>

      <div className="ep-filter-bar">
        <span className="ep-filter-label">查询</span>
        <Input
          placeholder="请输入平台订单号 (EP...)"
          value={orderNo}
          onChange={(e) => setOrderNo(e.target.value)}
          onPressEnter={load}
          style={{ width: 340 }}
          prefix={<SearchOutlined style={{ color: 'var(--text-tertiary)' }} />}
        />
        <Button type="primary" onClick={load}>查询</Button>
      </div>

      <Table
        rowKey="id"
        dataSource={list}
        pagination={false}
        scroll={{ x: 1400 }}
        columns={[
          {
            title: '编号',
            dataIndex: 'id',
            width: 80,
            render: (v: number) => <span className="mono">#{String(v).padStart(4, '0')}</span>,
          },
          {
            title: '事件',
            dataIndex: 'event_type',
            width: 160,
            render: (v: string) => <span className="mono" style={{ fontSize: 11 }}>{v}</span>,
          },
          {
            title: '回调地址',
            dataIndex: 'notify_url',
            ellipsis: true,
            width: 280,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 120,
            render: (s: string) => <Tag color={statusColor[s]}>{statusLabel[s] || s}</Tag>,
          },
          {
            title: '重试次数',
            dataIndex: 'retry_count',
            width: 90,
            render: (v: number) => <span className="mono">{v} 次</span>,
          },
          {
            title: 'HTTP 状态',
            dataIndex: 'http_status',
            width: 100,
            render: (v: number) => v
              ? <span className="mono" style={{
                  color: v >= 200 && v < 300 ? 'var(--accent-emerald)' : 'var(--accent-crimson)',
                }}>{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '下次重试',
            dataIndex: 'next_retry_at',
            width: 180,
            render: (v: string) => v
              ? <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '错误信息',
            dataIndex: 'last_error',
            width: 260,
            render: (v: string) => v
              ? (
                <Tooltip title={v}>
                  <span style={{ color: 'var(--accent-crimson)', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
                    {v.slice(0, 40)}{v.length > 40 ? '…' : ''}
                  </span>
                </Tooltip>
              )
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '创建时间',
            dataIndex: 'created_at',
            width: 180,
            render: (v: string) => <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v}</span>,
          },
          {
            title: '操作',
            width: 110,
            fixed: 'right',
            render: (_, row) => (
              row.status !== 'success'
                ? <Button size="small" icon={<RedoOutlined />} onClick={() => retry(row.id)}>重推</Button>
                : null
            ),
          },
        ]}
      />
    </>
  )
}
