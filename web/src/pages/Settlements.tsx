import { useEffect, useMemo, useState } from 'react'
import { Table, Tag, Select, Button, Modal, Input, InputNumber, DatePicker, message, Tooltip, Tabs } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { adminApi } from '../api'
import type { MerchantBalanceRow } from '../api'

const statusColor: Record<string, string> = { pending: 'orange', paid: 'green', cancelled: 'default' }
const statusLabel: Record<string, string> = { pending: '待打款', paid: '已打款', cancelled: '已取消' }

export default function Settlements() {
  const [tab, setTab] = useState('balances')
  const [balances, setBalances] = useState<MerchantBalanceRow[]>([])
  const [list, setList] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [filter, setFilter] = useState<{ status?: string }>({})
  const [settleTarget, setSettleTarget] = useState<MerchantBalanceRow | null>(null)
  const [feeRate, setFeeRate] = useState(0)
  const [period, setPeriod] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null)
  const [periodAmount, setPeriodAmount] = useState(0)
  const [periodIncome, setPeriodIncome] = useState(0)
  const [periodRefund, setPeriodRefund] = useState(0)
  const [remark, setRemark] = useState('')
  const [settling, setSettling] = useState(false)

  const loadBalances = async () => {
    try {
      const data = await adminApi.listMerchantBalances()
      setBalances(data ?? [])
    } catch {}
  }
  const loadHistory = async () => {
    const result = await adminApi.listSettlements({ page, size, ...filter })
    setList(result.list ?? [])
    setTotal(result.total ?? 0)
  }

  useEffect(() => { loadBalances() }, [])
  useEffect(() => { if (tab === 'history') loadHistory() }, [tab, page, filter])

  const stats = useMemo(() => {
    const totalAvailable = balances.reduce((s, b) => s + b.available, 0)
    const withBalance = balances.filter((b) => b.available > 0).length
    return { totalAvailable, withBalance, merchantCount: balances.length }
  }, [balances])

  const fetchPeriodBalance = async (merchantId: number, start: dayjs.Dayjs, end: dayjs.Dayjs) => {
    try {
      const pb = await adminApi.merchantPeriodBalance(merchantId, start.format('YYYY-MM-DD'), end.format('YYYY-MM-DD'))
      setPeriodAmount(pb.amount)
      setPeriodIncome(pb.income)
      setPeriodRefund(pb.refund)
    } catch {
      setPeriodAmount(0)
      setPeriodIncome(0)
      setPeriodRefund(0)
    }
  }

  const openSettle = (row: MerchantBalanceRow) => {
    setSettleTarget(row)
    setFeeRate(0)
    const p: [dayjs.Dayjs, dayjs.Dayjs] = [dayjs(row.period_start), dayjs()]
    setPeriod(p)
    setRemark('')
    fetchPeriodBalance(row.merchant_id, p[0], p[1])
  }

  const onPeriodChange = (v: [dayjs.Dayjs, dayjs.Dayjs] | null) => {
    setPeriod(v)
    if (v && settleTarget) {
      fetchPeriodBalance(settleTarget.merchant_id, v[0], v[1])
    } else {
      setPeriodAmount(0)
    }
  }

  const fee = Math.round(periodAmount * feeRate / 100)
  const netAmount = periodAmount - fee

  const submitSettle = async () => {
    if (!settleTarget || !period) return
    setSettling(true)
    try {
      await adminApi.createSettlement({
        merchant_id: settleTarget.merchant_id,
        fee_rate: feeRate / 100,
        period_start: period[0].format('YYYY-MM-DD'),
        period_end: period[1].format('YYYY-MM-DD'),
        remark,
      })
      message.success('结算单已创建')
      setSettleTarget(null)
      loadBalances()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '创建失败')
    } finally {
      setSettling(false)
    }
  }

  const markPaid = (id: number) => {
    Modal.confirm({
      title: '确认已打款？',
      content: '请确认已通过银行转账完成打款',
      okText: '确认',
      cancelText: '取消',
      centered: true,
      onOk: async () => {
        try {
          await adminApi.markSettlementPaid(id)
          message.success('已标记打款')
          loadHistory()
        } catch (e: any) {
          message.error(e.response?.data?.msg || '操作失败')
        }
      },
    })
  }

  const cancel = async (id: number) => {
    try {
      await adminApi.cancelSettlement(id)
      message.success('已取消')
      loadHistory()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '操作失败')
    }
  }

  return (
    <>
      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">商户数</div>
          <div className="value"><span className="mono">{String(stats.merchantCount).padStart(2, '0')}</span></div>
          <div className="trend">● 全部</div>
        </div>
        <div className="ep-stat">
          <div className="label">有余额商户</div>
          <div className="value"><span className="mono">{String(stats.withBalance).padStart(2, '0')}</span></div>
          <div className="trend">● 可结算</div>
        </div>
        <div className="ep-stat">
          <div className="label">平台待结算总额</div>
          <div className="value">¥<span className="mono">{(stats.totalAvailable / 100).toFixed(2)}</span></div>
          <div className="trend dim">● 人民币</div>
        </div>
      </div>

      <Tabs
        activeKey={tab}
        onChange={setTab}
        style={{ marginBottom: 0 }}
        items={[
          {
            key: 'balances',
            label: '商户余额',
            children: (
              <>
                <div className="ep-filter-bar" style={{ marginTop: 12 }}>
                  <div className="ep-filter-actions">
                    <Button icon={<ReloadOutlined />} onClick={loadBalances}>刷新</Button>
                  </div>
                </div>
                <div>
                  <Table
                    rowKey="merchant_id"
                    dataSource={balances}
                    pagination={false}
                    size="small"
                    scroll={{ x: 'max-content' }}
                    columns={[
                      { title: '商户号', dataIndex: 'mch_no', width: 180, render: (v: string) => <span className="tracked-id">{v}</span> },
                      { title: '商户名称', dataIndex: 'name', width: 150 },
                      { title: '总收入', dataIndex: 'total_income', width: 130, render: (v: number) => <span className="money">¥{(v / 100).toFixed(2)}</span> },
                      { title: '已退款', dataIndex: 'total_refund', width: 130, render: (v: number) => <span style={{ color: 'var(--text-secondary)' }}>¥{(v / 100).toFixed(2)}</span> },
                      { title: '已结算', dataIndex: 'total_settled', width: 130, render: (v: number) => <span style={{ color: 'var(--text-secondary)' }}>¥{(v / 100).toFixed(2)}</span> },
                      {
                        title: '可结算余额', dataIndex: 'available', width: 140,
                        render: (v: number) => <span className="money" style={{ color: v > 0 ? 'var(--accent-emerald)' : 'var(--text-faint)' }}>¥{(v / 100).toFixed(2)}</span>,
                      },
                      {
                        title: '操作', width: 100, fixed: 'right' as const,
                        render: (_: any, row: MerchantBalanceRow) => row.available > 0
                          ? <Button type="link" size="small" onClick={() => openSettle(row)}>结算</Button>
                          : <span style={{ color: 'var(--text-faint)', fontSize: 12 }}>无余额</span>,
                      },
                    ]}
                  />
                </div>
              </>
            ),
          },
          {
            key: 'history',
            label: '结算记录',
            children: (
              <>
                <div className="ep-filter-bar" style={{ marginTop: 12 }}>
                  <span className="ep-filter-label">筛选</span>
                  <Select
                    placeholder="状态"
                    allowClear
                    style={{ width: 150 }}
                    onChange={(v) => { setPage(1); setFilter({ status: v }) }}
                    options={[
                      { value: 'pending', label: '待打款' },
                      { value: 'paid', label: '已打款' },
                      { value: 'cancelled', label: '已取消' },
                    ]}
                  />
                  <div className="ep-filter-actions">
                    <Button icon={<ReloadOutlined />} onClick={loadHistory}>刷新</Button>
                  </div>
                </div>
                <div>
                  <Table
                    rowKey="id"
                    dataSource={list}
                    pagination={{ current: page, pageSize: size, total, onChange: setPage, showTotal: (t) => `共 ${t} 条` }}
                    size="small"
                    scroll={{ x: 'max-content' }}
                    columns={[
                      { title: '结算单号', dataIndex: 'settlement_no', width: 220, render: (v: string) => <span className="tracked-id">{v}</span> },
                      { title: '结算金额', dataIndex: 'amount', width: 120, render: (v: number) => <span className="money">¥{(v / 100).toFixed(2)}</span> },
                      { title: '手续费', dataIndex: 'fee', width: 100, render: (v: number) => <span style={{ color: 'var(--text-secondary)' }}>¥{(v / 100).toFixed(2)}</span> },
                      { title: '实付金额', dataIndex: 'net_amount', width: 120, render: (v: number) => <span className="money" style={{ color: 'var(--accent-emerald)' }}>¥{(v / 100).toFixed(2)}</span> },
                      { title: '状态', dataIndex: 'status', width: 100, render: (s: string) => <Tag color={statusColor[s] || 'default'}>{statusLabel[s] || s}</Tag> },
                      {
                        title: '打款时间', dataIndex: 'paid_at', width: 120,
                        render: (v: string) => v ? <Tooltip title={v}><span className="mono" style={{ fontSize: 11, color: 'var(--accent-emerald)' }}>{v.slice(0, 10)}</span></Tooltip> : <span style={{ color: 'var(--text-faint)' }}>—</span>,
                      },
                      {
                        title: '创建时间', dataIndex: 'created_at', width: 120,
                        render: (v: string) => <Tooltip title={v}><span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v?.slice(0, 10)}</span></Tooltip>,
                      },
                      { title: '备注', dataIndex: 'remark', width: 150, ellipsis: true, render: (v: string) => v || <span style={{ color: 'var(--text-faint)' }}>—</span> },
                      {
                        title: '操作', width: 140, fixed: 'right' as const,
                        render: (_: any, row: any) => row.status === 'pending' ? (
                          <span style={{ display: 'flex', gap: 4 }}>
                            <Button type="link" size="small" onClick={() => markPaid(row.id)}>确认打款</Button>
                            <Button type="link" size="small" danger onClick={() => cancel(row.id)}>取消</Button>
                          </span>
                        ) : null,
                      },
                    ]}
                  />
                </div>
              </>
            ),
          },
        ]}
      />

      <Modal
        title="确认结算"
        open={!!settleTarget}
        onOk={submitSettle}
        onCancel={() => setSettleTarget(null)}
        confirmLoading={settling}
        okText="确认结算"
        cancelText="取消"
        centered
        width={480}
      >
        {settleTarget && (
          <div>
            <div style={{ padding: '16px', background: 'var(--bg-elevated)', borderRadius: 8, marginBottom: 16 }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10 }}>{settleTarget.name} · {settleTarget.mch_no}</div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6, fontSize: 12, color: 'var(--text-secondary)' }}>
                <div>总收入：<span className="mono">¥{(settleTarget.total_income / 100).toFixed(2)}</span></div>
                <div>已退款：<span className="mono">¥{(settleTarget.total_refund / 100).toFixed(2)}</span></div>
                <div>已结算：<span className="mono">¥{(settleTarget.total_settled / 100).toFixed(2)}</span></div>
                <div>可结算：<span className="mono" style={{ color: 'var(--accent-emerald)' }}>¥{(settleTarget.available / 100).toFixed(2)}</span></div>
              </div>
            </div>

            <div style={{ marginBottom: 14 }}>
              <div style={{ fontSize: 12, color: 'var(--text-tertiary)', marginBottom: 6 }}>结算周期</div>
              <DatePicker.RangePicker
                style={{ width: '100%' }}
                value={period}
                onChange={(v) => onPeriodChange(v as [dayjs.Dayjs, dayjs.Dayjs])}
              />
            </div>

            <div style={{ marginBottom: 14 }}>
              <div style={{ fontSize: 12, color: 'var(--text-tertiary)', marginBottom: 6 }}>手续费率（%）</div>
              <InputNumber
                min={0} max={100} step={0.1}
                style={{ width: '100%' }}
                value={feeRate}
                onChange={(v) => setFeeRate(v ?? 0)}
                addonAfter="%"
              />
            </div>

            <div style={{ padding: '12px 16px', background: 'var(--bg-base)', borderRadius: 6, marginBottom: 14 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>
                <span>周期收入</span>
                <span className="mono">¥{(periodIncome / 100).toFixed(2)}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>
                <span>周期退款</span>
                <span className="mono">- ¥{(periodRefund / 100).toFixed(2)}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4, borderTop: '1px solid var(--border-hairline)', paddingTop: 4 }}>
                <span>结算金额</span>
                <span className="mono">¥{(periodAmount / 100).toFixed(2)}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>
                <span>手续费（{feeRate}%）</span>
                <span className="mono">- ¥{(fee / 100).toFixed(2)}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 14, fontWeight: 600, color: 'var(--accent-emerald)', borderTop: '1px solid var(--border-hairline)', paddingTop: 8, marginTop: 4 }}>
                <span>实付金额</span>
                <span className="mono">¥{(netAmount / 100).toFixed(2)}</span>
              </div>
            </div>

            <div>
              <div style={{ fontSize: 12, color: 'var(--text-tertiary)', marginBottom: 6 }}>备注（可选）</div>
              <Input.TextArea rows={2} value={remark} onChange={(e) => setRemark(e.target.value)} placeholder="可选" />
            </div>
          </div>
        )}
      </Modal>
    </>
  )
}
