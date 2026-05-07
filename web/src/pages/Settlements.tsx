import { useEffect, useMemo, useState } from 'react'
import { Table, Tag, Select, Button, Modal, Form, InputNumber, Input, DatePicker, message, Tooltip } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { adminApi } from '../api'
import type { Merchant } from '../api'

const statusColor: Record<string, string> = {
  pending: 'orange',
  paid: 'green',
  cancelled: 'default',
}
const statusLabel: Record<string, string> = {
  pending: '待打款',
  paid: '已打款',
  cancelled: '已取消',
}

function TimeCell({ value }: { value?: string }) {
  if (!value) return <span style={{ color: 'var(--text-faint)' }}>—</span>
  const short = value.slice(0, 10)
  return (
    <Tooltip title={value}>
      <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{short}</span>
    </Tooltip>
  )
}

export default function Settlements() {
  const [list, setList] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [filter, setFilter] = useState<{ status?: string; merchant_id?: string }>({})
  const [merchants, setMerchants] = useState<Merchant[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [balance, setBalance] = useState<any>(null)
  const [form] = Form.useForm()

  const load = async () => {
    const result = await adminApi.listSettlements({ page, size, ...filter })
    setList(result.list ?? [])
    setTotal(result.total ?? 0)
  }
  const loadMerchants = async () => {
    const result = await adminApi.listMerchants({ page: 1, size: 500 })
    setMerchants(result.list)
  }
  useEffect(() => { load() }, [page, filter])
  useEffect(() => { loadMerchants() }, [])

  const merchantMap = useMemo(() => {
    const m = new Map<number, Merchant>()
    merchants.forEach((x) => m.set(x.id, x))
    return m
  }, [merchants])

  const stats = useMemo(() => {
    const pendingList = list.filter((s) => s.status === 'pending')
    const paidList = list.filter((s) => s.status === 'paid')
    return {
      total,
      pendingCount: pendingList.length,
      paidCount: paidList.length,
      pendingAmount: (pendingList.reduce((s, r) => s + r.net_amount, 0) / 100).toFixed(2),
    }
  }, [list, total])

  const openCreate = async () => {
    if (merchants.length === 0) await loadMerchants()
    form.resetFields()
    setBalance(null)
    setCreateOpen(true)
  }

  const onMerchantSelect = async (merchantId: number) => {
    try {
      const bal = await adminApi.merchantBalance(merchantId)
      setBalance(bal)
    } catch { setBalance(null) }
  }

  const submitCreate = async () => {
    const v = await form.validateFields()
    setCreating(true)
    try {
      await adminApi.createSettlement({
        merchant_id: v.merchant_id,
        amount: v.amount,
        fee: v.fee || 0,
        period_start: v.period[0].toISOString(),
        period_end: v.period[1].toISOString(),
        remark: v.remark || '',
      })
      message.success('结算单已创建')
      setCreateOpen(false)
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '创建失败')
    } finally {
      setCreating(false)
    }
  }

  const markPaid = async (id: number) => {
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
          load()
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
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '操作失败')
    }
  }

  return (
    <>
      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">结算总数</div>
          <div className="value"><span className="mono">{String(stats.total).padStart(3, '0')}</span></div>
          <div className="trend">● 全部</div>
        </div>
        <div className="ep-stat">
          <div className="label">待打款</div>
          <div className="value"><span className="mono">{String(stats.pendingCount).padStart(2, '0')}</span></div>
          <div className="trend dim">○ 等待中</div>
        </div>
        <div className="ep-stat">
          <div className="label">已打款</div>
          <div className="value"><span className="mono">{String(stats.paidCount).padStart(2, '0')}</span></div>
          <div className="trend">● 已完成</div>
        </div>
        <div className="ep-stat">
          <div className="label">待打款金额</div>
          <div className="value">¥<span className="mono">{stats.pendingAmount}</span></div>
          <div className="trend dim">● 人民币</div>
        </div>
      </div>

      <div className="ep-filter-bar">
        <span className="ep-filter-label">筛选</span>
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 150 }}
          onChange={(v) => setFilter({ ...filter, status: v })}
          options={[
            { value: 'pending', label: '待打款' },
            { value: 'paid', label: '已打款' },
            { value: 'cancelled', label: '已取消' },
          ]}
        />
        <Select
          placeholder="商户"
          allowClear
          showSearch
          optionFilterProp="label"
          style={{ width: 220 }}
          onChange={(v) => setFilter({ ...filter, merchant_id: v ? String(v) : undefined })}
          options={merchants.map((m) => ({ value: m.id, label: `${m.name} · ${m.mch_no}` }))}
        />
        <div className="ep-filter-actions">
          <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建结算</Button>
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
            title: '商户',
            dataIndex: 'merchant_id',
            width: 150,
            ellipsis: true,
            render: (v: number) => {
              const m = merchantMap.get(v)
              return <Tooltip title={m ? `${m.name} · ${m.mch_no}` : `#${v}`}>
                <span>{m?.name || `#${v}`}</span>
              </Tooltip>
            },
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
            width: 120,
            render: (v: string) => <TimeCell value={v} />,
          },
          {
            title: '创建时间',
            dataIndex: 'created_at',
            width: 120,
            render: (v: string) => <TimeCell value={v} />,
          },
          {
            title: '备注',
            dataIndex: 'remark',
            width: 150,
            ellipsis: true,
            render: (v: string) => v || <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '操作',
            width: 140,
            fixed: 'right' as const,
            render: (_: any, row: any) => {
              if (row.status !== 'pending') return null
              return (
                <span style={{ display: 'flex', gap: 4 }}>
                  <Button type="link" size="small" onClick={() => markPaid(row.id)}>确认打款</Button>
                  <Button type="link" size="small" danger onClick={() => cancel(row.id)}>取消</Button>
                </span>
              )
            },
          },
        ]}
      />

      <Modal
        title="新建结算单"
        open={createOpen}
        onOk={submitCreate}
        onCancel={() => setCreateOpen(false)}
        confirmLoading={creating}
        okText="创建"
        cancelText="取消"
        centered
      >
        <Form form={form} layout="vertical" requiredMark={false}>
          <Form.Item name="merchant_id" label="商户" rules={[{ required: true }]}>
            <Select
              placeholder="选择商户"
              showSearch
              optionFilterProp="label"
              options={merchants.map((m) => ({ value: m.id, label: `${m.mch_no} · ${m.name}` }))}
              onChange={onMerchantSelect}
            />
          </Form.Item>
          {balance && (
            <div style={{ marginBottom: 16, padding: '12px 16px', background: 'var(--bg-card)', borderRadius: 8, fontSize: 12, color: 'var(--text-tertiary)' }}>
              <div>总收入：¥{(balance.total_income / 100).toFixed(2)}</div>
              <div>已退款：¥{(balance.total_refund / 100).toFixed(2)}</div>
              <div>已结算：¥{(balance.total_settled / 100).toFixed(2)}</div>
              <div style={{ color: 'var(--accent-emerald)', fontWeight: 600 }}>可结算：¥{(balance.available / 100).toFixed(2)}</div>
            </div>
          )}
          <Form.Item name="period" label="结算周期" rules={[{ required: true }]}>
            <DatePicker.RangePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="amount" label="结算金额（分）" rules={[{ required: true }]}
            extra={balance ? `最大可结 ${balance.available} 分` : ''}>
            <InputNumber min={1} max={balance?.available} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="fee" label="手续费（分）" initialValue={0}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}
