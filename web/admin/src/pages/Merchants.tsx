import { useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, Space, Tag } from 'antd'
import { PlusOutlined, SettingOutlined } from '@ant-design/icons'
import { api } from '../api'
import ChannelConfigDrawer from '../components/ChannelConfigDrawer'

interface Merchant {
  id: number
  mch_no: string
  name: string
  app_id: string
  notify_url: string
  status: number
  remark: string
}

export default function Merchants() {
  const [list, setList] = useState<Merchant[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [open, setOpen] = useState(false)
  const [channelTarget, setChannelTarget] = useState<Merchant | null>(null)
  const [form] = Form.useForm()

  const load = async () => {
    const { data } = await api.get('/admin/merchants', { params: { page, size } })
    setList(data.data.list)
    setTotal(data.data.total)
  }
  useEffect(() => { load() }, [page])

  const create = async () => {
    const v = await form.validateFields()
    const { data } = await api.post('/admin/merchants', v)
    Modal.success({
      title: '商户创建成功',
      width: 560,
      content: (
        <div>
          <p style={{ color: 'var(--text-secondary)', marginBottom: 16 }}>
            请妥善保存 <code>app_secret</code>，它只会在创建时展示一次。
          </p>
          <pre style={{
            background: 'var(--bg-deep)',
            border: '1px solid var(--border-hairline)',
            padding: 16,
            fontFamily: 'var(--font-mono)',
            fontSize: 12,
            color: 'var(--accent-gold)',
            margin: 0,
          }}>
            app_id     : {data.data.app_id}
            {'\n'}app_secret : {data.data.app_secret}
          </pre>
        </div>
      ),
    })
    setOpen(false)
    form.resetFields()
    load()
  }

  const activeCount = list.filter((m) => m.status === 1).length

  return (
    <>
      <div className="ep-page-header">
        <div className="col-title">
          <div className="eyebrow">商户 · 01</div>
          <h1>商户<em>名册</em></h1>
        </div>
        <div className="col-actions">
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
            新建商户
          </Button>
        </div>
      </div>

      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">商户总数</div>
          <div className="value"><span className="mono">{String(total).padStart(2, '0')}</span></div>
          <div className="trend dim">● 已注册</div>
        </div>
        <div className="ep-stat">
          <div className="label">启用中</div>
          <div className="value"><span className="mono">{String(activeCount).padStart(2, '0')}</span></div>
          <div className="trend">● 运行中</div>
        </div>
        <div className="ep-stat">
          <div className="label">当前页</div>
          <div className="value"><span className="mono">{String(page).padStart(2, '0')}</span><span className="unit">/ {Math.max(1, Math.ceil(total / size))}</span></div>
          <div className="trend dim">● 分页</div>
        </div>
        <div className="ep-stat">
          <div className="label">每页条数</div>
          <div className="value"><span className="mono">{size}</span></div>
          <div className="trend dim">● 默认</div>
        </div>
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
        columns={[
          {
            title: '编号',
            dataIndex: 'id',
            width: 80,
            render: (v: number) => <span className="mono">#{String(v).padStart(4, '0')}</span>,
          },
          {
            title: '商户号',
            dataIndex: 'mch_no',
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '商户名称',
            dataIndex: 'name',
            render: (v: string) => <span className="title-cell">{v}</span>,
          },
          {
            title: '应用 ID',
            dataIndex: 'app_id',
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '回调地址',
            dataIndex: 'notify_url',
            ellipsis: true,
            render: (v: string) => v
              ? <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 110,
            render: (s: number) => s === 1
              ? <Tag color="green">● 启用</Tag>
              : <Tag color="default">○ 停用</Tag>,
          },
          {
            title: '操作',
            width: 140,
            render: (_, row) => (
              <Space>
                <Button size="small" icon={<SettingOutlined />} onClick={() => setChannelTarget(row)}>
                  渠道配置
                </Button>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title="新建商户"
        open={open}
        onOk={create}
        onCancel={() => setOpen(false)}
        okText="创建商户"
        cancelText="取消"
      >
        <Form form={form} layout="vertical" requiredMark={false}>
          <Form.Item name="mch_no" label="商户号" rules={[{ required: true, message: '请输入商户号' }]}>
            <Input placeholder="如 M100001" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="商户展示名称" />
          </Form.Item>
          <Form.Item name="notify_url" label="下游 Notify URL">
            <Input placeholder="https://your-service/callback" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} placeholder="对内备注（选填）" />
          </Form.Item>
        </Form>
      </Modal>

      <ChannelConfigDrawer
        merchantId={channelTarget?.id ?? null}
        merchantName={channelTarget?.name ?? ''}
        open={!!channelTarget}
        onClose={() => setChannelTarget(null)}
      />
    </>
  )
}
