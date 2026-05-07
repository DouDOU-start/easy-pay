import { useEffect, useMemo, useState } from 'react'
import { Table, Tag, Select, Input, Button, Modal, Form, InputNumber, message, Typography, Tooltip } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { QRCodeSVG } from 'qrcode.react'
import { adminApi } from '../api'

const { Text, Paragraph } = Typography

function TimeCell({ value, accent }: { value?: string; accent?: boolean }) {
  if (!value) return <span style={{ color: 'var(--text-faint)' }}>—</span>
  const short = value.slice(0, 19).replace('T', ' ')
  return (
    <Tooltip title={value}>
      <span
        className="mono"
        style={{ fontSize: 11, color: accent ? 'var(--accent-emerald)' : 'var(--text-secondary)', whiteSpace: 'nowrap' }}
      >
        {short}
      </span>
    </Tooltip>
  )
}

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

interface Merchant {
  id: number
  mch_no: string
  name: string
}

interface CreatedOrder {
  order_no: string
  merchant_order_no: string
  code_url: string
  h5_url: string
}

export default function Orders() {
  const [list, setList] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [filter, setFilter] = useState<{ status?: string; channel?: string; merchant_id?: string }>({})

  const [merchants, setMerchants] = useState<Merchant[]>([])
  const [merchantChannels, setMerchantChannels] = useState<{ channel: string; status: number }[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<CreatedOrder | null>(null)
  const [form] = Form.useForm()
  const [refundOpen, setRefundOpen] = useState(false)
  const [refunding, setRefunding] = useState(false)
  const [refundTarget, setRefundTarget] = useState<any>(null)
  const [refundForm] = Form.useForm()

  const load = async () => {
    const result = await adminApi.listOrders({ page, size, ...filter })
    setList(result.list)
    setTotal(result.total)
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

  const channelLabel: Record<string, string> = { wechat: '微信支付', alipay: '支付宝' }

  const loadMerchantChannels = async (merchantId: number) => {
    const channels = await adminApi.listMerchantChannels(merchantId)
    const enabled = (channels || []).filter((c: any) => c.status === 1)
    setMerchantChannels(enabled)
    return enabled
  }

  const onMerchantChange = async (merchantId: number) => {
    form.setFieldsValue({ channel: undefined, trade_type: undefined })
    setMerchantChannels([])
    if (merchantId) {
      const chs = await loadMerchantChannels(merchantId)
      if (chs.length === 1) {
        form.setFieldsValue({ channel: chs[0].channel })
      }
    }
  }

  const openCreate = async () => {
    if (merchants.length === 0) await loadMerchants()
    form.resetFields()
    setMerchantChannels([])
    form.setFieldsValue({
      trade_type: 'native',
      amount: 1,
      subject: 'easy-pay 测试订单',
      expire_seconds: 900,
    })
    setCreateOpen(true)
  }

  const submitCreate = async () => {
    const v = await form.validateFields()
    setCreating(true)
    try {
      const result = await adminApi.createTestOrder(v)
      setCreated(result as CreatedOrder)
      setCreateOpen(false)
      message.success('下单成功')
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '下单失败')
    } finally {
      setCreating(false)
    }
  }

  const openRefund = (record: any) => {
    setRefundTarget(record)
    refundForm.setFieldsValue({ amount: record.amount, reason: '' })
    setRefundOpen(true)
  }

  const submitRefund = async () => {
    const v = await refundForm.validateFields()
    setRefunding(true)
    try {
      await adminApi.refundOrder(refundTarget.order_no, { amount: v.amount, reason: v.reason })
      message.success('退款请求已提交')
      setRefundOpen(false)
      setRefundTarget(null)
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '退款失败')
    } finally {
      setRefunding(false)
    }
  }

  const stats = useMemo(() => {
    const paidList = list.filter((o) => o.status === 'paid')
    const pendingCount = list.filter((o) => o.status === 'pending').length
    const paidAmount = paidList.reduce((s, o) => s + (o.amount || 0), 0)
    return {
      total,
      paidCount: paidList.length,
      pendingCount,
      paidAmount: (paidAmount / 100).toFixed(2),
    }
  }, [list, total])

  return (
    <>
      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">订单总数</div>
          <div className="value"><span className="mono">{String(stats.total).padStart(3, '0')}</span></div>
          <div className="trend">● 实时</div>
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
          onChange={(v) => setFilter({ ...filter, status: v })}
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
          onChange={(v) => setFilter({ ...filter, channel: v })}
          options={[
            { value: 'wechat', label: '微信' },
            { value: 'alipay', label: '支付宝' },
          ]}
        />
        <Select
          placeholder="商户"
          allowClear
          showSearch
          style={{ width: 220 }}
          optionFilterProp="label"
          value={filter.merchant_id ? Number(filter.merchant_id) : undefined}
          onChange={(v) => setFilter({ ...filter, merchant_id: v ? String(v) : undefined })}
          options={merchants.map((m) => ({ value: m.id, label: `${m.name} · ${m.mch_no}` }))}
        />
        <div className="ep-filter-actions">
          <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建测试订单</Button>
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
            width: 200,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '商户',
            dataIndex: 'merchant_id',
            width: 150,
            ellipsis: true,
            render: (v: number) => {
              const m = merchantMap.get(v)
              const name = m?.name || `#${v}`
              const tip = m ? `${m.name} · ${m.mch_no}` : `#${v}`
              return (
                <Tooltip title={tip}>
                  <span style={{ color: 'var(--text-primary)' }}>{name}</span>
                </Tooltip>
              )
            },
          },
          {
            title: '渠道',
            dataIndex: 'channel',
            width: 90,
            render: (v: string) => <span style={{ fontSize: 12 }}>{{ wechat: '微信', alipay: '支付宝' }[v] || v}</span>,
          },
          {
            title: '类型',
            dataIndex: 'trade_type',
            width: 90,
            render: (v: string) => <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{{ native: '扫码', h5: 'H5' }[v] || v}</span>,
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
            render: (s: string, row: any) => {
              const expired = s === 'pending' && row.expire_at && new Date(row.expire_at) < new Date()
              if (expired) {
                return <Tag color="default">已过期</Tag>
              }
              if (s === 'pending' && (row.code_url || row.h5_url)) {
                return (
                  <Tag
                    color={statusColor[s]}
                    style={{ cursor: 'pointer' }}
                    onClick={() => setCreated({
                      order_no: row.order_no,
                      merchant_order_no: row.merchant_order_no,
                      code_url: row.code_url ?? '',
                      h5_url: row.h5_url ?? '',
                    })}
                  >
                    {statusLabel[s]}
                  </Tag>
                )
              }
              return <Tag color={statusColor[s] || 'default'}>{statusLabel[s] || s}</Tag>
            },
          },
          {
            title: '渠道单号',
            dataIndex: 'channel_order_no',
            width: 200,
            render: (v: string) => v
              ? <span className="tracked-id">{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
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
          {
            title: '操作',
            width: 100,
            fixed: 'right' as const,
            render: (_: any, row: any) => {
              if (row.status === 'paid' || row.status === 'partial_refunded') {
                return <Button type="link" size="small" danger onClick={() => openRefund(row)}>退款</Button>
              }
              return null
            },
          },
        ]}
      />

      <Modal
        title="新建测试订单"
        open={createOpen}
        onOk={submitCreate}
        onCancel={() => setCreateOpen(false)}
        confirmLoading={creating}
        okText="提交下单"
        cancelText="取消"
        centered
      >
        <Form form={form} layout="vertical" requiredMark={false}>
          <Form.Item name="merchant_id" label="商户" rules={[{ required: true }]}>
            <Select
              placeholder="选择商户"
              options={merchants.map((m) => ({ value: m.id, label: `${m.mch_no} · ${m.name}` }))}
              onChange={onMerchantChange}
            />
          </Form.Item>
          <Form.Item name="channel" label="渠道" rules={[{ required: true }]}>
            <Select
              placeholder={form.getFieldValue('merchant_id') ? '选择渠道' : '请先选择商户'}
              disabled={merchantChannels.length === 0}
              options={merchantChannels.map((c) => ({ value: c.channel, label: channelLabel[c.channel] || c.channel }))}
            />
          </Form.Item>
          <Form.Item name="trade_type" label="下单类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'native', label: 'Native 扫码' },
              { value: 'h5', label: 'H5' },
            ]} />
          </Form.Item>
          <Form.Item name="subject" label="商品描述" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="amount"
            label="金额（分）"
            rules={[{ required: true }]}
            extra="1 = ¥0.01，测试建议用最小金额"
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="expire_seconds" label="过期时间（秒）">
            <InputNumber min={60} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="支付信息"
        open={!!created}
        onCancel={() => setCreated(null)}
        footer={<Button onClick={() => setCreated(null)}>关闭</Button>}
        width={520}
        centered
      >
        {created && (
          <div>
            <Paragraph style={{ marginBottom: 10 }}>
              <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>平台单号</Text>
              <br />
              <Text code copyable>{created.order_no}</Text>
            </Paragraph>
            <Paragraph style={{ marginBottom: 18 }}>
              <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>商户单号</Text>
              <br />
              <Text code copyable>{created.merchant_order_no}</Text>
            </Paragraph>
            {created.code_url && (
              <>
                <Paragraph style={{ marginBottom: 10 }}>
                  <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>扫码支付</Text>
                </Paragraph>
                <div className="ep-qr-wrap">
                  <QRCodeSVG value={created.code_url} size={220} includeMargin />
                </div>
                <div style={{
                  marginTop: 12,
                  fontSize: 11,
                  color: 'var(--text-tertiary)',
                  textAlign: 'center',
                  letterSpacing: '0.1em',
                }}>
                  使用微信「扫一扫」完成支付
                </div>
                <Paragraph style={{ marginTop: 16 }} copyable={{ text: created.code_url }}>
                  <Text type="secondary" style={{ fontSize: 11, wordBreak: 'break-all', fontFamily: 'var(--font-mono)' }}>
                    {created.code_url}
                  </Text>
                </Paragraph>
              </>
            )}
            {created.h5_url && (
              <Paragraph copyable={{ text: created.h5_url }}>
                <Text style={{ color: 'var(--text-tertiary)', fontSize: 11, letterSpacing: '0.1em' }}>H5 支付链接</Text>
                <br />
                <Text code style={{ fontSize: 11, wordBreak: 'break-all' }}>{created.h5_url}</Text>
              </Paragraph>
            )}
          </div>
        )}
      </Modal>

      <Modal
        title="订单退款"
        open={refundOpen}
        onOk={submitRefund}
        onCancel={() => { setRefundOpen(false); setRefundTarget(null) }}
        confirmLoading={refunding}
        okText="确认退款"
        okButtonProps={{ danger: true }}
        cancelText="取消"
        centered
      >
        {refundTarget && (
          <div style={{ marginBottom: 16, padding: '12px 16px', background: 'var(--bg-card)', borderRadius: 8 }}>
            <div style={{ fontSize: 12, color: 'var(--text-tertiary)', marginBottom: 4 }}>
              平台单号：<span className="mono">{refundTarget.order_no}</span>
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-tertiary)' }}>
              订单金额：<span className="money">¥{(refundTarget.amount / 100).toFixed(2)}</span>
            </div>
          </div>
        )}
        <Form form={refundForm} layout="vertical" requiredMark={false}>
          <Form.Item
            name="amount"
            label="退款金额（分）"
            rules={[
              { required: true, message: '请输入退款金额' },
              {
                validator: (_, val) =>
                  val && refundTarget && val > refundTarget.amount
                    ? Promise.reject('退款金额不能超过订单金额')
                    : Promise.resolve(),
              },
            ]}
            extra={refundTarget ? `最大可退 ${refundTarget.amount} 分（¥${(refundTarget.amount / 100).toFixed(2)}）` : ''}
          >
            <InputNumber min={1} max={refundTarget?.amount} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="reason" label="退款原因">
            <Input.TextArea rows={2} placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}
