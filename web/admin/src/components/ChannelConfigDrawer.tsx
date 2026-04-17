import { useEffect, useState } from 'react'
import { Drawer, Form, Switch, Button, Tag, Divider, message, Alert } from 'antd'
import { api } from '../api'

interface Props {
  merchantId: number | null
  merchantName: string
  open: boolean
  onClose: () => void
}

interface ChannelRow {
  id: number
  channel: 'wechat' | 'alipay'
  status: number
  updated_at: string
}

export default function ChannelConfigDrawer({ merchantId, merchantName, open, onClose }: Props) {
  const [rows, setRows] = useState<Record<string, ChannelRow>>({})
  const [saving, setSaving] = useState<string | null>(null)
  const [wechatForm] = Form.useForm()
  const [alipayForm] = Form.useForm()

  const formOf = (ch: 'wechat' | 'alipay') => (ch === 'wechat' ? wechatForm : alipayForm)

  const load = async () => {
    if (!merchantId) return
    try {
      const { data } = await api.get(`/admin/merchants/${merchantId}/channels`)
      const map: Record<string, ChannelRow> = {}
      for (const row of (data.data ?? []) as ChannelRow[]) map[row.channel] = row
      setRows(map)
      for (const ch of ['wechat', 'alipay'] as const) {
        const row = map[ch]
        formOf(ch).setFieldsValue({ enabled: row ? row.status === 1 : false })
      }
    } catch {
      setRows({})
    }
  }

  useEffect(() => {
    if (open && merchantId) load()
  }, [open, merchantId])

  const save = async (ch: 'wechat' | 'alipay') => {
    if (!merchantId) return
    const v = await formOf(ch).validateFields()
    setSaving(ch)
    try {
      await api.put(`/admin/merchants/${merchantId}/channels/${ch}`, {
        status: v.enabled ? 1 : 0,
      })
      message.success(`${ch === 'wechat' ? '微信支付' : '支付宝'}已保存`)
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg ?? '保存失败')
    } finally {
      setSaving(null)
    }
  }

  const statusTag = (ch: 'wechat' | 'alipay') => {
    const row = rows[ch]
    if (!row) return <Tag>未开通</Tag>
    return row.status === 1
      ? <Tag color="green">已启用 · {row.updated_at?.slice(0, 16).replace('T', ' ')}</Tag>
      : <Tag color="orange">已停用</Tag>
  }

  const ChannelSection = ({ ch, label }: { ch: 'wechat' | 'alipay'; label: string }) => (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16, gap: 12 }}>
        <span style={{ fontWeight: 600, fontSize: 15 }}>{label}</span>
        {statusTag(ch)}
      </div>
      <Form form={formOf(ch)} layout="vertical" initialValues={{ enabled: false }}>
        <Form.Item name="enabled" label="启用渠道" valuePropName="checked">
          <Switch checkedChildren="已启用" unCheckedChildren="停用" />
        </Form.Item>
        <Button type="primary" loading={saving === ch} onClick={() => save(ch)}>
          保存 {label}
        </Button>
      </Form>
    </div>
  )

  return (
    <Drawer
      title={
        <div>
          <div style={{ fontSize: 11, letterSpacing: '0.15em', color: 'var(--accent-gold)', marginBottom: 8 }}>
            渠道授权
          </div>
          <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, color: 'var(--text-primary)', lineHeight: 1 }}>
            {merchantName || '商户'}
          </div>
        </div>
      }
      open={open}
      onClose={onClose}
      width={480}
      destroyOnClose
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 24 }}
        message="此处只控制该商户对各渠道的使用权限。渠道证书和密钥请到左侧「渠道凭证」统一配置。"
      />

      <ChannelSection ch="wechat" label="微信支付" />
      <Divider />
      <ChannelSection ch="alipay" label="支付宝" />
    </Drawer>
  )
}
