import { useEffect, useState } from 'react'
import { Drawer, Switch, Tag, Divider, message, Alert, Empty, Spin } from 'antd'
import { adminApi } from '../api'

interface Props {
  merchantId: number | null
  merchantName: string
  open: boolean
  onClose: () => void
}

interface ChannelRow {
  channel: string
  status: number
  updated_at: string
}

interface PlatformChannel {
  channel: string
  status: number
  configured: boolean
}

const channelLabel: Record<string, string> = { wechat: '微信支付', alipay: '支付宝' }

export default function ChannelConfigDrawer({ merchantId, merchantName, open, onClose }: Props) {
  const [merchantChs, setMerchantChs] = useState<Record<string, ChannelRow>>({})
  const [platformChs, setPlatformChs] = useState<PlatformChannel[]>([])
  const [saving, setSaving] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const load = async () => {
    if (!merchantId) return
    setLoading(true)
    try {
      const [mList, pList] = await Promise.all([
        adminApi.listMerchantChannels(merchantId),
        adminApi.listPlatformChannels(),
      ])
      const map: Record<string, ChannelRow> = {}
      for (const row of (mList ?? []) as unknown as ChannelRow[]) map[row.channel] = row
      setMerchantChs(map)
      setPlatformChs((pList ?? []).filter((p: PlatformChannel) => p.status === 1 && p.configured))
    } catch {
      setMerchantChs({})
      setPlatformChs([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open && merchantId) load()
  }, [open, merchantId])

  const toggle = async (ch: string, enabled: boolean) => {
    if (!merchantId) return
    setSaving(ch)
    try {
      await adminApi.upsertMerchantChannel(merchantId, ch, {
        status: enabled ? 1 : 0,
      })
      message.success(`${channelLabel[ch] ?? ch} ${enabled ? '已启用' : '已停用'}`)
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg ?? '保存失败')
    } finally {
      setSaving(null)
    }
  }

  const statusTag = (ch: string) => {
    const row = merchantChs[ch]
    if (!row) return <Tag>未开通</Tag>
    return row.status === 1
      ? <Tag color="green">已启用 · {row.updated_at?.slice(0, 16).replace('T', ' ')}</Tag>
      : <Tag color="orange">已停用</Tag>
  }

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

      {loading ? (
        <div style={{ textAlign: 'center', padding: '48px 0' }}><Spin /></div>
      ) : platformChs.length === 0 ? (
        <Empty
          description="平台尚未配置任何可用渠道，请先在「渠道凭证」中完成配置。"
          style={{ marginTop: 48 }}
        />
      ) : (
        platformChs.map((pc, i) => (
          <div key={pc.channel}>
            {i > 0 && <Divider />}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <span style={{ fontWeight: 600, fontSize: 15 }}>{channelLabel[pc.channel] ?? pc.channel}</span>
                {statusTag(pc.channel)}
              </div>
              <Switch
                checked={merchantChs[pc.channel]?.status === 1}
                loading={saving === pc.channel}
                checkedChildren="已启用"
                unCheckedChildren="停用"
                onChange={(checked) => toggle(pc.channel, checked)}
              />
            </div>
          </div>
        ))
      )}
    </Drawer>
  )
}
