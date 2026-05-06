import { useEffect, useState } from 'react'
import { Button, Input, Select, Table, Tag, Tooltip } from 'antd'
import { RedoOutlined, SearchOutlined } from '@ant-design/icons'
import { merchantApi } from '../api'

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

export default function MerchantNotifyLogs() {
  const [orderNo, setOrderNo] = useState('')
  const [status, setStatus] = useState<string | undefined>(undefined)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [total, setTotal] = useState(0)
  const [list, setList] = useState<any[]>([])

  const load = async () => {
    const params: Record<string, any> = { page, size }
    if (orderNo.trim()) params.order_no = orderNo.trim()
    if (status) params.status = status
    const { data } = await merchantApi.get('/merchant/notify_logs', { params })
    setList(data.data.list ?? [])
    setTotal(data.data.total ?? 0)
  }

  useEffect(() => { load() }, [page, status])

  const onSearch = () => {
    if (page !== 1) setPage(1)
    else load()
  }

  const resetFilters = () => {
    setOrderNo('')
    setStatus(undefined)
    if (page !== 1) setPage(1)
  }

  return (
    <>
      <div className="ep-page-header">
        <div className="col-title">
          <div className="eyebrow">Notify</div>
          <h1>只看当前商户自己的<em>回调记录</em>。</h1>
          <div className="subtitle">按平台订单号和投递状态过滤，方便核对下游通知是否成功送达。</div>
        </div>
      </div>

      <div className="ep-filter-bar">
        <span className="ep-filter-label">筛选</span>
        <Input
          placeholder="平台订单号 (EP...)"
          value={orderNo}
          onChange={(e) => setOrderNo(e.target.value)}
          onPressEnter={onSearch}
          allowClear
          style={{ width: 320 }}
          prefix={<SearchOutlined style={{ color: 'var(--text-tertiary)' }} />}
        />
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 140 }}
          value={status}
          onChange={(v) => setStatus(v)}
          options={[
            { value: 'pending', label: '待投递' },
            { value: 'success', label: '已成功' },
            { value: 'failed', label: '已失败' },
            { value: 'dropped', label: '已丢弃' },
          ]}
        />
        <Button type="primary" onClick={onSearch}>查询</Button>
        <Button onClick={resetFilters}>重置</Button>
      </div>

      <Table
        rowKey="id"
        dataSource={list}
        sticky
        pagination={{
          current: page,
          pageSize: size,
          total,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        scroll={{ x: 'max-content', y: '100%' }}
        columns={[
          {
            title: '事件',
            dataIndex: 'event_type',
            width: 160,
            render: (v: string) => <span className="mono" style={{ fontSize: 11 }}>{v}</span>,
          },
          {
            title: '平台订单号',
            dataIndex: 'order_no',
            width: 220,
            render: (v: string) => <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v}</span>,
          },
          {
            title: '回调地址',
            dataIndex: 'notify_url',
            ellipsis: true,
            width: 300,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 110,
            render: (s: string) => <Tag color={statusColor[s]}>{statusLabel[s] || s}</Tag>,
          },
          {
            title: '重试',
            dataIndex: 'retry_count',
            width: 90,
            render: (v: number) => <span className="mono">{v} 次</span>,
          },
          {
            title: 'HTTP',
            dataIndex: 'http_status',
            width: 80,
            render: (v: number) => v
              ? <span className="mono" style={{ color: v >= 200 && v < 300 ? 'var(--accent-emerald)' : 'var(--accent-crimson)' }}>{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '下次重试',
            dataIndex: 'next_retry_at',
            width: 200,
            render: (v: string) => v
              ? <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{v.slice(0, 19).replace('T', ' ')}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '错误信息',
            dataIndex: 'last_error',
            width: 240,
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
            width: 200,
            render: (v: string) => <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{v.slice(0, 19).replace('T', ' ')}</span>,
          },
          {
            title: '说明',
            width: 120,
            fixed: 'right',
            render: (_, row) => row.status !== 'success' ? <span style={{ color: 'var(--text-secondary)' }}><RedoOutlined /> 待平台继续重试</span> : <span style={{ color: 'var(--accent-emerald)' }}>已投递</span>,
          },
        ]}
      />
    </>
  )
}
