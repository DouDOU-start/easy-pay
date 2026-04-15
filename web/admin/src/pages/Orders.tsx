import { useEffect, useMemo, useState } from 'react'
import { Table, Tag, Select, Input, Button, Modal, Form, InputNumber, message, Typography } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { QRCodeSVG } from 'qrcode.react'
import { api } from '../api'

const { Text, Paragraph } = Typography

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
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<CreatedOrder | null>(null)
  const [form] = Form.useForm()

  const load = async () => {
    const { data } = await api.get('/admin/orders', {
      params: { page, size, ...filter },
    })
    setList(data.data.list)
    setTotal(data.data.total)
  }
  useEffect(() => { load() }, [page, filter])

  const openCreate = async () => {
    const { data } = await api.get('/admin/merchants', { params: { page: 1, size: 100 } })
    setMerchants(data.data.list)
    form.resetFields()
    form.setFieldsValue({
      channel: 'wechat',
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
      const { data } = await api.post('/admin/orders/test', v)
      setCreated(data.data)
      setCreateOpen(false)
      message.success('下单成功')
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '下单失败')
    } finally {
      setCreating(false)
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
      <div className="ep-page-header">
        <div className="col-title">
          <div className="eyebrow">交易 · 02</div>
          <h1>订单<em>流水</em></h1>
        </div>
        <div className="col-actions">
          <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建测试订单</Button>
        </div>
      </div>

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
        <Input
          placeholder="商户 ID"
          style={{ width: 150 }}
          onChange={(e) => setFilter({ ...filter, merchant_id: e.target.value })}
        />
      </div>

      <Table
        rowKey="id"
        dataSource={list}
        pagination={{
          current: page,
          pageSize: size,
          total,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        scroll={{ x: 1300 }}
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
            width: 80,
            render: (v: number) => <span className="mono">#{v}</span>,
          },
          {
            title: '渠道',
            dataIndex: 'channel',
            width: 90,
            render: (v: string) => <span className="mono" style={{ textTransform: 'uppercase', fontSize: 11 }}>{v}</span>,
          },
          {
            title: '类型',
            dataIndex: 'trade_type',
            width: 90,
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
            render: (v: string) => v
              ? <span className="tracked-id">{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '创建时间',
            dataIndex: 'created_at',
            width: 180,
            render: (v: string) => <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v}</span>,
          },
          {
            title: '支付时间',
            dataIndex: 'paid_at',
            width: 180,
            render: (v: string) => v
              ? <span className="mono" style={{ fontSize: 11, color: 'var(--accent-emerald)' }}>{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
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
      >
        <Form form={form} layout="vertical" requiredMark={false}>
          <Form.Item name="merchant_id" label="商户" rules={[{ required: true }]}>
            <Select
              placeholder="选择商户"
              options={merchants.map((m) => ({ value: m.id, label: `${m.mch_no} · ${m.name}` }))}
            />
          </Form.Item>
          <Form.Item name="channel" label="渠道" rules={[{ required: true }]}>
            <Select options={[
              { value: 'wechat', label: '微信支付' },
              { value: 'alipay', label: '支付宝（占位）' },
            ]} />
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
        title="下单成功"
        open={!!created}
        onCancel={() => setCreated(null)}
        footer={<Button onClick={() => setCreated(null)}>关闭</Button>}
        width={520}
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
    </>
  )
}
