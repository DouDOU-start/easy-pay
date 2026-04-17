import { useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, Select, Switch, Space, Tag, message, Typography } from 'antd'
import { PlusOutlined, SettingOutlined, EditOutlined, SearchOutlined, CopyOutlined, KeyOutlined } from '@ant-design/icons'
import { api } from '../api'
import ChannelConfigDrawer from '../components/ChannelConfigDrawer'

interface Merchant {
  id: number
  mch_no: string
  name: string
  email: string
  app_id: string
  notify_url: string
  status: number
  remark: string
}

function CredRow({ label, value, sensitive }: { label: string; value: string; sensitive?: boolean }) {
  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: '110px 1fr auto',
      alignItems: 'center',
      gap: 12,
      padding: '10px 14px',
      borderBottom: '1px solid var(--border-hairline)',
      background: sensitive ? 'rgba(232, 96, 96, 0.04)' : 'transparent',
    }}>
      <span style={{
        fontSize: 11,
        letterSpacing: '0.08em',
        color: 'var(--text-tertiary)',
        textTransform: 'uppercase',
        fontFamily: 'var(--font-mono)',
      }}>
        {label}
      </span>
      <Typography.Text
        copyable={false}
        style={{
          fontFamily: 'var(--font-mono)',
          fontSize: 12,
          color: sensitive ? 'var(--accent-crimson)' : 'var(--accent-gold)',
          wordBreak: 'break-all',
          margin: 0,
        }}
      >
        {value}
      </Typography.Text>
      <Typography.Text
        copyable={{ text: value, tooltips: ['复制', '已复制'] }}
        style={{ color: 'var(--text-tertiary)' }}
      />
    </div>
  )
}

export default function Merchants() {
  const [list, setList] = useState<Merchant[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<number | undefined>(undefined)
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Merchant | null>(null)
  const [channelTarget, setChannelTarget] = useState<Merchant | null>(null)
  const [createForm] = Form.useForm()
  const [editForm] = Form.useForm()

  const load = async () => {
    const params: Record<string, any> = { page, size }
    if (keyword.trim()) params.keyword = keyword.trim()
    if (status !== undefined) params.status = status
    const { data } = await api.get('/admin/merchants', { params })
    setList(data.data.list)
    setTotal(data.data.total)
  }
  useEffect(() => { load() }, [page, status])

  const onSearch = () => {
    if (page !== 1) setPage(1)
    else load()
  }

  const resetFilters = () => {
    setKeyword('')
    setStatus(undefined)
    if (page !== 1) setPage(1)
  }

  const create = async () => {
    const v = await createForm.validateFields()
    const { data } = await api.post('/admin/merchants', v)
    const d = data.data
    const copyAll = () => {
      const text = `邮箱: ${d.email}\n初始密码: ${d.password}\n商户号: ${d.mch_no}\napp_id: ${d.app_id}\napp_secret: ${d.app_secret}`
      navigator.clipboard.writeText(text).then(() => message.success('全部信息已复制'))
    }
    Modal.success({
      title: '商户创建成功',
      width: 600,
      icon: null,
      okText: '我已保存',
      content: (
        <div>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            padding: '10px 14px',
            marginBottom: 18,
            background: 'rgba(217, 184, 112, 0.08)',
            border: '1px solid rgba(217, 184, 112, 0.3)',
            fontSize: 12,
            color: 'var(--accent-gold)',
          }}>
            <span>⚠</span>
            <span>初始密码与 app_secret 仅在此处展示一次，请立即保存到安全位置。</span>
          </div>

          <CredRow label="登录邮箱" value={d.email} />
          <CredRow label="初始密码" value={d.password} sensitive />
          <CredRow label="商户号" value={d.mch_no} />
          <CredRow label="app_id" value={d.app_id} />
          <CredRow label="app_secret" value={d.app_secret} sensitive />

          <Button
            block
            icon={<CopyOutlined />}
            onClick={copyAll}
            style={{ marginTop: 8 }}
          >
            一键复制全部
          </Button>
        </div>
      ),
    })
    setCreateOpen(false)
    createForm.resetFields()
    load()
  }

  const resetPassword = (m: Merchant) => {
    Modal.confirm({
      title: '重置登录密码',
      content: (
        <div style={{ color: 'var(--text-secondary)' }}>
          确定要为商户 <span style={{ color: 'var(--accent-gold)', fontFamily: 'var(--font-mono)' }}>{m.email || m.mch_no}</span> 生成新的随机密码吗？旧密码将立即失效。
        </div>
      ),
      okText: '重置',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        const { data } = await api.post(`/admin/merchants/${m.id}/reset-password`)
        const d = data.data
        const copyAll = () => {
          const text = `邮箱: ${d.email}\n新密码: ${d.password}`
          navigator.clipboard.writeText(text).then(() => message.success('已复制'))
        }
        Modal.success({
          title: '密码已重置',
          width: 560,
          icon: null,
          okText: '我已保存',
          content: (
            <div>
              <div style={{
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                padding: '10px 14px',
                marginBottom: 18,
                background: 'rgba(217, 184, 112, 0.08)',
                border: '1px solid rgba(217, 184, 112, 0.3)',
                fontSize: 12,
                color: 'var(--accent-gold)',
              }}>
                <span>⚠</span>
                <span>新密码仅在此处展示一次，请立即交付给商户。</span>
              </div>
              <CredRow label="登录邮箱" value={d.email} />
              <CredRow label="新密码" value={d.password} sensitive />
              <Button block icon={<CopyOutlined />} onClick={copyAll} style={{ marginTop: 8 }}>
                一键复制
              </Button>
            </div>
          ),
        })
      },
    })
  }

  const openEdit = (m: Merchant) => {
    setEditTarget(m)
    editForm.setFieldsValue({
      name: m.name,
      notify_url: m.notify_url,
      remark: m.remark,
      enabled: m.status === 1,
    })
  }

  const saveEdit = async () => {
    if (!editTarget) return
    const v = await editForm.validateFields()
    try {
      await api.put(`/admin/merchants/${editTarget.id}`, {
        name: v.name,
        notify_url: v.notify_url ?? '',
        remark: v.remark ?? '',
        status: v.enabled ? 1 : 0,
      })
      message.success('已保存')
      setEditTarget(null)
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg ?? '保存失败')
    }
  }

  const activeCount = list.filter((m) => m.status === 1).length

  return (
    <>
      <div className="ep-stat-strip" style={{ gridTemplateColumns: 'repeat(2, 1fr)' }}>
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
      </div>

      <div className="ep-filter-bar">
        <span className="ep-filter-label">筛选</span>
        <Input
          placeholder="搜索商户号 / 名称 / 应用 ID"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
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
            { value: 1, label: '启用' },
            { value: 0, label: '停用' },
          ]}
        />
        <Button type="primary" onClick={onSearch}>查询</Button>
        <Button onClick={resetFilters}>重置</Button>
        <div className="ep-filter-actions">
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateOpen(true)}
          >
            新建商户
          </Button>
        </div>
      </div>

      <Table
        rowKey="id"
        dataSource={list}
        sticky
        scroll={{ y: '100%' }}
        pagination={{
          current: page,
          pageSize: size,
          total,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
        columns={[
          {
            title: '商户号',
            dataIndex: 'mch_no',
            ellipsis: true,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '商户名称',
            dataIndex: 'name',
            ellipsis: true,
            render: (v: string) => <span className="title-cell">{v}</span>,
          },
          {
            title: '邮箱',
            dataIndex: 'email',
            ellipsis: true,
            render: (v: string) => v
              ? <span className="mono" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{v}</span>
              : <span style={{ color: 'var(--text-faint)' }}>—</span>,
          },
          {
            title: '应用 ID',
            dataIndex: 'app_id',
            ellipsis: true,
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
            width: 100,
            render: (s: number) => s === 1
              ? <Tag color="green">● 启用</Tag>
              : <Tag color="default">○ 停用</Tag>,
          },
          {
            title: '操作',
            width: 180,
            render: (_, row) => (
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
                  编辑
                </Button>
                <Button size="small" icon={<SettingOutlined />} onClick={() => setChannelTarget(row)}>
                  渠道
                </Button>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title="新建商户"
        open={createOpen}
        onOk={create}
        onCancel={() => setCreateOpen(false)}
        okText="创建商户"
        cancelText="取消"
      >
        <Form form={createForm} layout="vertical" requiredMark={false}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="商户展示名称" />
          </Form.Item>
          <Form.Item
            name="email"
            label="登录邮箱"
            rules={[
              { required: true, message: '请输入登录邮箱' },
              { type: 'email', message: '请输入有效邮箱' },
            ]}
            extra="新建时会自动生成初始密码，创建成功后只展示一次。"
          >
            <Input placeholder="merchant@example.com" />
          </Form.Item>
          <Form.Item name="notify_url" label="下游 Notify URL">
            <Input placeholder="https://your-service/callback" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} placeholder="对内备注（选填）" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={editTarget ? `编辑商户 · ${editTarget.mch_no}` : '编辑商户'}
        open={!!editTarget}
        onOk={saveEdit}
        onCancel={() => setEditTarget(null)}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={editForm} layout="vertical" requiredMark={false}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="商户展示名称" />
          </Form.Item>
          <Form.Item name="notify_url" label="下游 Notify URL">
            <Input placeholder="https://your-service/callback" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} placeholder="对内备注（选填）" />
          </Form.Item>
          <Form.Item
            name="enabled"
            label="状态"
            valuePropName="checked"
            extra="停用后商户将无法调用支付 API。"
          >
            <Switch checkedChildren="启用" unCheckedChildren="停用" />
          </Form.Item>
        </Form>

        {editTarget && (
          <div style={{
            marginTop: 8,
            paddingTop: 16,
            borderTop: '1px solid var(--border-hairline)',
          }}>
            <div style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 16,
            }}>
              <div>
                <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4 }}>
                  登录密码
                </div>
                <div style={{ fontSize: 11, color: 'var(--text-tertiary)', lineHeight: 1.6 }}>
                  生成 12 位随机新密码，旧密码立即失效。
                </div>
              </div>
              <Button danger icon={<KeyOutlined />} onClick={() => resetPassword(editTarget)}>
                重置密码
              </Button>
            </div>
          </div>
        )}
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
