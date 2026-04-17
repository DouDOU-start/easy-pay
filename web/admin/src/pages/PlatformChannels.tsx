import { useEffect, useState } from 'react'
import {
  Table, Button, Drawer, Tabs, Form, Input, Switch, Upload, message, Alert, Space, Tag,
} from 'antd'
import { SettingOutlined, UploadOutlined } from '@ant-design/icons'
import type { UploadProps } from 'antd'
import { api } from '../api'

interface PlatformChannel {
  id: number
  channel: 'wechat' | 'alipay'
  status: number
  updated_at: string
}

// Sentinel: server sends this for already-configured secret fields so we can
// pre-fill the form without leaking plaintext. Mirrors channelKeepSentinel in
// internal/handler/admin/admin.go.
const KEEP = '__KEEP__'
const isKept = (v: unknown) => v === KEEP

async function readFileText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result ?? ''))
    r.onerror = () => reject(r.error)
    r.readAsText(file)
  })
}

export default function PlatformChannels() {
  const [list, setList] = useState<PlatformChannel[]>([])
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [tab, setTab] = useState<'wechat' | 'alipay'>('wechat')
  const [wechatForm] = Form.useForm()
  const [alipayForm] = Form.useForm()

  const load = async () => {
    try {
      const { data } = await api.get('/admin/platform/channels')
      setList(data.data ?? [])
    } catch {
      setList([])
    }
  }

  const loadChannelConfig = async (ch: 'wechat' | 'alipay') => {
    const form = ch === 'wechat' ? wechatForm : alipayForm
    try {
      const { data } = await api.get(`/admin/platform/channels/${ch}`)
      if (!data.data) return
      form.setFieldsValue(data.data.config ?? {})
    } catch {
      // no prior config — leave the form empty
    }
  }

  const openDrawer = () => {
    wechatForm.resetFields()
    alipayForm.resetFields()
    alipayForm.setFieldValue('sign_type', 'RSA2')
    loadChannelConfig('wechat')
    loadChannelConfig('alipay')
    setDrawerOpen(true)
  }

  useEffect(() => { load() }, [])

  const saveWechat = async () => {
    const v = await wechatForm.validateFields()
    setSaving(true)
    try {
      await api.put('/admin/platform/channels/wechat', {
        config: {
          mch_id: v.mch_id.trim(),
          app_id: v.app_id.trim(),
          api_v3_key: v.api_v3_key.trim(),
          serial_no: v.serial_no.trim(),
          private_key_pem: v.private_key_pem,
          public_key_id: (v.public_key_id ?? '').trim(),
          public_key_pem: v.public_key_pem,
        },
      })
      message.success('微信支付凭证已保存')
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg ?? '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const saveAlipay = async () => {
    const v = await alipayForm.validateFields()
    setSaving(true)
    try {
      await api.put('/admin/platform/channels/alipay', {
        config: {
          app_id: v.app_id.trim(),
          private_key: v.private_key,
          alipay_public_key: v.alipay_public_key,
          sign_type: v.sign_type || 'RSA2',
          is_production: !!v.is_production,
        },
      })
      message.success('支付宝凭证已保存')
      load()
    } catch (e: any) {
      message.error(e.response?.data?.msg ?? '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const pemUpload = (field: string, form: ReturnType<typeof Form.useForm>[0]): UploadProps => ({
    accept: '.pem,.txt,.key',
    showUploadList: false,
    beforeUpload: async (file) => {
      try {
        form.setFieldValue(field, await readFileText(file))
        message.success(`已读入 ${file.name}`)
      } catch (e: any) {
        message.error(`读取失败: ${e.message}`)
      }
      return false
    },
  })

  // Batch import — same logic as the old ChannelConfigDrawer.
  const batchUpload: UploadProps = {
    multiple: true,
    showUploadList: false,
    accept: '.pem,.txt,.key,.crt',
    beforeUpload: async (file) => {
      if (!(file as any)._epProcessed) {
        ;(file as any)._epProcessed = true
        try {
          const text = await readFileText(file)
          const applied = await applyImportedFile(file.name, text)
          if (applied) message.success(`${file.name} → ${applied}`)
          else message.warning(`${file.name} 未识别，已跳过`)
        } catch (e: any) {
          message.error(`${file.name} 读取失败: ${e.message}`)
        }
      }
      return false
    },
  }

  async function applyImportedFile(name: string, content: string): Promise<string> {
    if (content.includes('BEGIN PUBLIC KEY')) {
      wechatForm.setFieldValue('public_key_pem', content)
      return '微信支付公钥 → public_key_pem'
    }
    if (content.includes('BEGIN PRIVATE KEY') || content.includes('BEGIN RSA PRIVATE KEY')) {
      wechatForm.setFieldValue('private_key_pem', content)
      return '商户私钥 → private_key_pem'
    }
    if (content.includes('BEGIN CERTIFICATE')) {
      try {
        const { data } = await api.post('/admin/wechat/parse-cert', { pem: content })
        wechatForm.setFieldValue('serial_no', data.data.serial_no)
        if (data.data.subject && /^\d{8,16}$/.test(data.data.subject)) {
          if (!wechatForm.getFieldValue('mch_id')) {
            wechatForm.setFieldValue('mch_id', data.data.subject)
          }
        }
        return `商户证书 → serial_no (${String(data.data.serial_no).slice(0, 12)}…)`
      } catch (e: any) {
        message.error('证书解析失败: ' + (e.response?.data?.msg ?? e.message))
        return ''
      }
    }
    const lower = name.toLowerCase()
    if (lower.includes('apiv3') || lower.includes('密钥')) {
      const trimmed = content.trim()
      if (/^[\x20-\x7e]{32}$/.test(trimmed)) {
        wechatForm.setFieldValue('api_v3_key', trimmed)
        return 'APIv3 密钥 → api_v3_key'
      }
    }
    if (lower.endsWith('.p12')) return '.p12 不需要（已跳过）'
    return ''
  }

  const configured = list.filter((c) => c.status === 1).length

  return (
    <>
      <div className="ep-stat-strip">
        <div className="ep-stat">
          <div className="label">可用渠道</div>
          <div className="value">
            <span className="mono">{String(configured).padStart(2, '0')}</span>
            <span className="unit">/ 02</span>
          </div>
          <div className="trend">● 已启用</div>
        </div>
        <div className="ep-stat">
          <div className="label">支持渠道</div>
          <div className="value"><span className="mono">02</span></div>
          <div className="trend dim">● 微信 · 支付宝</div>
        </div>
      </div>

      <Table
        rowKey="channel"
        dataSource={[
          list.find((c) => c.channel === 'wechat') ?? { channel: 'wechat', status: 0, updated_at: '' },
          list.find((c) => c.channel === 'alipay') ?? { channel: 'alipay', status: 0, updated_at: '' },
        ]}
        pagination={false}
        scroll={{ x: 720 }}
        columns={[
          {
            title: '渠道',
            dataIndex: 'channel',
            width: 140,
            render: (v: string) => (
              <span className="title-cell">
                {v === 'wechat' ? '微信支付' : '支付宝'}
              </span>
            ),
          },
          {
            title: '标识',
            dataIndex: 'channel',
            width: 120,
            render: (v: string) => <span className="tracked-id">{v}</span>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 140,
            render: (s: number, row: any) =>
              row.updated_at
                ? s === 1
                  ? <Tag color="green">● 已配置</Tag>
                  : <Tag color="orange">○ 已停用</Tag>
                : <Tag>未配置</Tag>,
          },
          {
            title: '最后更新',
            dataIndex: 'updated_at',
            width: 180,
            render: (v: string) =>
              v ? (
                <span className="mono" style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
                  {v.slice(0, 16).replace('T', ' ')}
                </span>
              ) : (
                <span style={{ color: 'var(--text-faint)' }}>—</span>
              ),
          },
          {
            title: '操作',
            width: 100,
            render: (_, row: any) => (
              <Button
                size="small"
                icon={<SettingOutlined />}
                onClick={() => { setTab(row.channel); openDrawer() }}
              >
                配置
              </Button>
            ),
          },
        ]}
      />

      <Drawer
        title={
          <div>
            <div style={{ fontSize: 11, letterSpacing: '0.15em', color: 'var(--accent-gold)', marginBottom: 8 }}>
              平台凭证
            </div>
            <div style={{ fontFamily: 'var(--font-display)', fontSize: 30, color: 'var(--text-primary)', lineHeight: 1 }}>
              渠道配置
            </div>
          </div>
        }
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={700}
        destroyOnClose
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 20 }}
          message="这里是平台级凭证，所有下游商户共用。修改后立即生效，请确认无误后保存。"
        />

        <Tabs
          activeKey={tab}
          onChange={(k) => setTab(k as 'wechat' | 'alipay')}
          items={[
            {
              key: 'wechat',
              label: '微信支付',
              children: (
                <>
                  <Alert
                    type="info"
                    showIcon
                    style={{ marginBottom: 16 }}
                    message="全部字段在 商户平台 → 账户中心 → API 安全 里可以找到。"
                  />

                  <div style={{
                    marginBottom: 20, padding: 16,
                    border: '1px dashed var(--border-hairline, #444)',
                    borderRadius: 6, background: 'rgba(154,130,74,0.06)',
                  }}>
                    <div style={{ marginBottom: 8, fontWeight: 500 }}>📁 批量导入证书文件</div>
                    <div style={{ fontSize: 12, color: 'var(--text-secondary, #888)', marginBottom: 12 }}>
                      一次性选择 <code>apiclient_cert.pem</code>、<code>apiclient_key.pem</code>、<code>APIV3密钥.txt</code>
                      以及微信支付公钥 .pem，easy-pay 会自动识别类型填入对应字段。
                    </div>
                    <Upload {...batchUpload}>
                      <Button icon={<UploadOutlined />}>选择多个文件导入</Button>
                    </Upload>
                  </div>

                  <Form form={wechatForm} layout="vertical" autoComplete="off">
                    <Form.Item name="mch_id" label="商户号 mch_id"
                      rules={[{ required: true }, { pattern: /^\d{8,16}$/, message: '应为 8-16 位数字' }]}
                      extra="商户平台『账户中心 → 商户信息』">
                      <Input placeholder="1900000000" />
                    </Form.Item>
                    <Form.Item name="app_id" label="AppID" rules={[{ required: true }]}
                      extra="绑定的公众号/小程序 AppID，通常以 wx 开头">
                      <Input placeholder="wx + 16 位字符" />
                    </Form.Item>
                    <Form.Item name="api_v3_key" label="APIv3 密钥"
                      rules={[
                        { required: true },
                        { validator: (_, v) => !v || isKept(v) || String(v).length === 32 ? Promise.resolve() : Promise.reject(new Error('必须正好 32 个字符')) },
                      ]}
                      extra="账户中心 → API 安全 → APIv3 密钥（已配置时显示 __KEEP__，不改留着即可）">
                      <Input.Password placeholder="32 个字符" />
                    </Form.Item>
                    <Form.Item name="serial_no" label="证书序列号 serial_no" rules={[{ required: true }]}
                      extra="API 证书列表里的 40 位十六进制序列号">
                      <Input placeholder="ABCDEF0123456789..." />
                    </Form.Item>
                    <Form.Item
                      name="private_key_pem"
                      label={
                        <Space>
                          商户 API 私钥 (apiclient_key.pem)
                          <Upload {...pemUpload('private_key_pem', wechatForm)}>
                            <Button size="small" icon={<UploadOutlined />}>选择文件</Button>
                          </Upload>
                        </Space>
                      }
                      rules={[
                        { required: true },
                        { validator: (_, v) => !v || isKept(v) || (String(v).includes('BEGIN') && String(v).includes('PRIVATE KEY')) ? Promise.resolve() : Promise.reject(new Error('不是合法 PEM 私钥')) },
                      ]}
                    >
                      <Input.TextArea rows={10} placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----" style={{ fontFamily: 'monospace', fontSize: 12 }} />
                    </Form.Item>
                    <Form.Item name="public_key_id" label="微信支付公钥 ID (public_key_id)"
                      rules={[{ required: true }, { pattern: /^PUB_KEY_ID_/, message: '格式应为 PUB_KEY_ID_ 开头' }]}
                      extra="账户中心 → API 安全 → 验证微信支付身份 → 微信支付公钥">
                      <Input placeholder="PUB_KEY_ID_0117..." />
                    </Form.Item>
                    <Form.Item
                      name="public_key_pem"
                      label={
                        <Space>
                          微信支付公钥 (wxp_pub.pem)
                          <Upload {...pemUpload('public_key_pem', wechatForm)}>
                            <Button size="small" icon={<UploadOutlined />}>选择文件</Button>
                          </Upload>
                        </Space>
                      }
                      rules={[
                        { required: true },
                        { validator: (_, v) => !v || isKept(v) || (String(v).includes('BEGIN') && String(v).includes('PUBLIC KEY')) ? Promise.resolve() : Promise.reject(new Error('不是合法 PEM 公钥')) },
                      ]}
                    >
                      <Input.TextArea rows={8} placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----" style={{ fontFamily: 'monospace', fontSize: 12 }} />
                    </Form.Item>
                    <Button type="primary" loading={saving} onClick={saveWechat}>
                      保存微信凭证
                    </Button>
                  </Form>
                </>
              ),
            },
            {
              key: 'alipay',
              label: '支付宝',
              children: (
                <>
                  <Alert
                    type="warning"
                    showIcon
                    style={{ marginBottom: 16 }}
                    message="支付宝 SDK 尚未完整接入，配置可提前填好。"
                  />
                  <Form form={alipayForm} layout="vertical" autoComplete="off">
                    <Form.Item name="app_id" label="AppID" rules={[{ required: true }]}
                      extra="支付宝开放平台应用 AppID">
                      <Input placeholder="2021000000000000" />
                    </Form.Item>
                    <Form.Item
                      name="private_key"
                      label={
                        <Space>
                          应用私钥
                          <Upload {...pemUpload('private_key', alipayForm)}>
                            <Button size="small" icon={<UploadOutlined />}>选择文件</Button>
                          </Upload>
                        </Space>
                      }
                      rules={[{ required: true }]}
                    >
                      <Input.TextArea rows={6} style={{ fontFamily: 'monospace', fontSize: 12 }} />
                    </Form.Item>
                    <Form.Item
                      name="alipay_public_key"
                      label={
                        <Space>
                          支付宝公钥
                          <Upload {...pemUpload('alipay_public_key', alipayForm)}>
                            <Button size="small" icon={<UploadOutlined />}>选择文件</Button>
                          </Upload>
                        </Space>
                      }
                      rules={[{ required: true }]}
                    >
                      <Input.TextArea rows={6} style={{ fontFamily: 'monospace', fontSize: 12 }} />
                    </Form.Item>
                    <Form.Item name="sign_type" label="签名算法">
                      <Input />
                    </Form.Item>
                    <Form.Item name="is_production" label="正式环境" valuePropName="checked">
                      <Switch checkedChildren="正式" unCheckedChildren="沙箱" />
                    </Form.Item>
                    <Button type="primary" loading={saving} onClick={saveAlipay}>
                      保存支付宝凭证
                    </Button>
                  </Form>
                </>
              ),
            },
          ]}
        />
      </Drawer>
    </>
  )
}
