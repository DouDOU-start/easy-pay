import { useEffect, useMemo, useState } from 'react'
import { Table, Tag, Select, Button, Tooltip } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { merchantApi } from '../api'

const statusColor: Record<string, string> = { pending: 'orange', paid: 'green', cancelled: 'default' }
const statusLabel: Record<string, string> = { pending: '待打款', paid: '已打款', cancelled: '已取消' }

export default function MerchantSettlements() {
  const [list, setList] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [status, setStatus] = useState<string | undefined>()
  const [balance, setBalance] = useState<any>(null)

  const load = async () => {
    const params: Record<string, any> = { page, size }
    if (status) params.status = status
    const result = await merchantApi.listSettlements(params)
    setList(result.list ?? [])
    setTotal(result.total ?? 0)
  }
  const loadBalance = async () => {
    try {
      const bal = await merchantApi.balance()
      setBalance(bal)
    } catch {}
  }
  useEffect(() => { load() }, [page, status])
  useEffect(() => { loadBalance() }, [])

  const stats = useMemo(() => {
    if (!balance) return { income: '0.00', refund: '0.00', settled: '0.00', available: '0.00' }
    return {
      income: (balance.total_income / 100).toFixed(2),
      refund: (balance.total_refund / 100).toFixed(2),
      settled: (balance.total_settled / 100).toFixed(2),
      available: (balance.available / 100).toFixed(2),
    }
  }, [balance])

  return (
    <>
      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">总收入</div>
          <div className="value">¥<span className="mono">{stats.income}</span></div>
          <div className="trend">● 累计</div>
        </div>
        <div className="ep-stat">
          <div className="label">已退款</div>
          <div className="value">¥<span className="mono">{stats.refund}</span></div>
          <div className="trend dim">● 已扣减</div>
        </div>
        <div className="ep-stat">
          <div className="label">已结算</div>
          <div className="value">¥<span className="mono">{stats.settled}</span></div>
          <div className="trend">● 已打款</div>
        </div>
        <div className="ep-stat">
          <div className="label">可结算余额</div>
          <div className="value">¥<span className="mono">{stats.available}</span></div>
          <div className="trend dim">● 待结算</div>
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
            { value: 'pending', label: '待打款' },
            { value: 'paid', label: '已打款' },
            { value: 'cancelled', label: '已取消' },
          ]}
        />
        <div className="ep-filter-actions">
          <Button icon={<ReloadOutlined />} onClick={() => { load(); loadBalance() }}>刷新</Button>
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
            title: '结算单号',
            dataIndex: 'settlement_no',
            width: 240,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '结算金额',
            dataIndex: 'amount',
            width: 120,
            render: (v: number) => <span className="money">¥{(v / 100).toFixed(2)}</span>,
          },
          {
            title: '手续费',
            dataIndex: 'fee',
            width: 100,
            render: (v: number) => <span style={{ color: 'var(--text-secondary)' }}>¥{(v / 100).toFixed(2)}</span>,
          },
          {
            title: '实付金额',
            dataIndex: 'net_amount',
            width: 120,
            render: (v: number) => <span className="money" style={{ color: 'var(--accent-emerald)' }}>¥{(v / 100).toFixed(2)}</span>,
          },
          {
            title: '结算周期',
            width: 200,
            render: (_: any, row: any) => (
              <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>
                {row.period_start?.slice(0, 10)} ~ {row.period_end?.slice(0, 10)}
              </span>
            ),
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
            render: (s: string) => <Tag color={statusColor[s] || 'default'}>{statusLabel[s] || s}</Tag>,
          },
          {
            title: '打款时间',
            dataIndex: 'paid_at',
            width: 150,
            render: (v: string) => v
              ? <Tooltip title={v}><span className="mono" style={{ fontSize: 11, color: 'var(--accent-emerald)' }}>{v.slice(0, 10)}</span></Tooltip>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '创建时间',
            dataIndex: 'created_at',
            width: 150,
            render: (v: string) => <Tooltip title={v}><span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v?.slice(0, 10)}</span></Tooltip>,
          },
          {
            title: '备注',
            dataIndex: 'remark',
            width: 150,
            ellipsis: true,
            render: (v: string) => v || <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
        ]}
      />
    </>
  )
}
