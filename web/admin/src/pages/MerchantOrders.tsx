import { useEffect, useMemo, useState } from 'react'
import { Button, Select, Table, Tag, Tooltip } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { merchantApi } from '../api'

const statusColor: Record<string, string> = {
  pending: 'orange',
  paid: 'green',
  closed: 'default',
  refunded: 'purple',
  partial_refunded: 'purple',
  failed: 'red',
}

const statusLabel: Record<string, string> = {
  pending: '待支付',
  paid: '已支付',
  closed: '已关闭',
  refunded: '已退款',
  partial_refunded: '部分退款',
  failed: '失败',
}

function TimeCell({ value, accent }: { value?: string; accent?: boolean }) {
  if (!value) return <span style={{ color: 'var(--text-faint)' }}>—</span>
  const short = value.slice(0, 19).replace('T', ' ')
  return (
    <Tooltip title={value}>
      <span className="mono" style={{ fontSize: 11, color: accent ? 'var(--accent-emerald)' : 'var(--text-secondary)', whiteSpace: 'nowrap' }}>
        {short}
      </span>
    </Tooltip>
  )
}

export default function MerchantOrders() {
  const [list, setList] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [status, setStatus] = useState<string | undefined>(undefined)
  const [channel, setChannel] = useState<string | undefined>(undefined)

  const load = async () => {
    const params: Record<string, any> = { page, size }
    if (status) params.status = status
    if (channel) params.channel = channel
    const { data } = await merchantApi.get('/merchant/orders', { params })
    setList(data.data.list ?? [])
    setTotal(data.data.total ?? 0)
  }

  useEffect(() => { load() }, [page, status, channel])

  const stats = useMemo(() => {
    const paidList = list.filter((o) => o.status === 'paid')
    const pendingCount = list.filter((o) => o.status === 'pending').length
    const paidAmount = paidList.reduce((sum, o) => sum + (o.amount || 0), 0)
    return {
      total,
      paidCount: paidList.length,
      pendingCount,
      paidAmount: (paidAmount / 100).toFixed(2),
    }
  }, [list, total])

  return (
    <>
      <div className="ep-page-header">
        <div className="col-title">
          <div className="eyebrow">Bills</div>
          <h1>只看属于<em>当前商户</em>的订单。</h1>
          <div className="subtitle">账单列表自动按登录商户隔离，无法查看其他商户的订单与交易状态。</div>
        </div>
      </div>

      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">订单总数</div>
          <div className="value"><span className="mono">{String(stats.total).padStart(3, '0')}</span></div>
          <div className="trend">● 当前商户</div>
        </div>
        <div className="ep-stat">
          <div className="label">本页已支付</div>
          <div className="value"><span className="mono">{String(stats.paidCount).padStart(2, '0')}</span></div>
          <div className="trend">● 已结算</div>
        </div>
        <div className="ep-stat">
          <div className="label">本页待支付</div>
          <div className="value"><span className="mono">{String(stats.pendingCount).padStart(2, '0')}</span></div>
          <div className="trend dim">○ 等待中</div>
        </div>
        <div className="ep-stat">
          <div className="label">本页支付金额</div>
          <div className="value">¥<span className="mono">{stats.paidAmount}</span></div>
          <div className="trend dim">● 人民币</div>
        </div>
      </div>

      <div className="ep-filter-bar">
        <span className="ep-filter-label">筛选</span>
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 150 }}
          value={status}
          onChange={(v) => { setPage(1); setStatus(v) }}
          options={[
            { value: 'pending', label: '待支付' },
            { value: 'paid', label: '已支付' },
            { value: 'closed', label: '已关闭' },
            { value: 'refunded', label: '已退款' },
            { value: 'failed', label: '失败' },
          ]}
        />
        <Select
          placeholder="渠道"
          allowClear
          style={{ width: 150 }}
          value={channel}
          onChange={(v) => { setPage(1); setChannel(v) }}
          options={[
            { value: 'wechat', label: '微信' },
            { value: 'alipay', label: '支付宝' },
          ]}
        />
        <div className="ep-filter-actions">
          <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
        </div>
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
            title: '平台单号',
            dataIndex: 'order_no',
            width: 240,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '商户单号',
            dataIndex: 'merchant_order_no',
            width: 220,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '渠道',
            dataIndex: 'channel',
            width: 100,
            render: (v: string) => <span className="mono" style={{ textTransform: 'uppercase', fontSize: 11 }}>{v}</span>,
          },
          {
            title: '类型',
            dataIndex: 'trade_type',
            width: 100,
            render: (v: string) => <span className="mono" style={{ textTransform: 'uppercase', fontSize: 11, color: 'var(--text-secondary)' }}>{v}</span>,
          },
          {
            title: '金额',
            dataIndex: 'amount',
            width: 130,
            render: (v: number) => <span className="money">¥{(v / 100).toFixed(2)}</span>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 130,
            render: (s: string) => <Tag color={statusColor[s] || 'default'}>{statusLabel[s] || s}</Tag>,
          },
          {
            title: '渠道单号',
            dataIndex: 'channel_order_no',
            width: 200,
            render: (v: string) => v ? <span className="tracked-id">{v}</span> : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '创建时间',
            dataIndex: 'created_at',
            width: 200,
            render: (v: string) => <TimeCell value={v} />,
          },
          {
            title: '支付时间',
            dataIndex: 'paid_at',
            width: 200,
            render: (v: string) => <TimeCell value={v} accent />,
          },
        ]}
      />
    </>
  )
}
