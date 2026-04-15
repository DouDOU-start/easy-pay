import { useEffect, useState } from 'react'
import { Drawer, Tabs, Form, Input, Button, Upload, message, Alert, Space, Switch, Tag } from 'antd'
import { UploadOutlined } from '@ant-design/icons'
import type { UploadProps } from 'antd'
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

// Sentinel the backend ships for already-configured secret fields so we can
// prefill the form without leaking plaintext. Mirrors channelKeepSentinel in
// internal/handler/admin/admin.go — if you rename it, rename it there too.
const KEEP = '__KEEP__'
const isKept = (v: unknown) => v === KEEP

// Read a selected file as UTF-8 text and resolve with the content.
// Used to slurp apiclient_key.pem into the form field without asking the
// user to escape newlines manually.
async function readFileText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result ?? ''))
    r.onerror = () => reject(r.error)
    r.readAsText(file)
  })
}

export default function ChannelConfigDrawer({ merchantId, merchantName, open, onClose }: Props) {
  const [tab, setTab] = useState<'wechat' | 'alipay'>('wechat')
  const [configured, setConfigured] = useState<Record<string, ChannelRow>>({})
  const [saving, setSaving] = useState(false)
  const [wechatForm] = Form.useForm()
  const [alipayForm] = Form.useForm()

  const loadConfigured = async () => {
    if (!merchantId) return
    try {
      const { data } = await api.get(`/admin/merchants/${merchantId}/channels`)
      const map: Record<string, ChannelRow> = {}
      for (const row of data.data as ChannelRow[]) map[row.channel] = row
      setConfigured(map)
    } catch {
      setConfigured({})
    }
  }

  // Pull the currently-stored config for one channel and replay it into the
  // matching form. Secrets come back as the KEEP sentinel; the form shows
  // them as-is and the save path swaps them out on the server.
  const loadChannelConfig = async (ch: 'wechat' | 'alipay') => {
    if (!merchantId) return
    const form = ch === 'wechat' ? wechatForm : alipayForm
    try {
      const { data } = await api.get(`/admin/merchants/${merchantId}/channels/${ch}`)
      if (!data.data) return
      form.setFieldsValue(data.data.config || {})
    } catch {
      // no-op: a missing row just leaves the form empty
    }
  }

  useEffect(() => {
    if (open && merchantId) {
      loadConfigured()
      wechatForm.resetFields()
      alipayForm.resetFields()
      alipayForm.setFieldsValue({ sign_type: 'RSA2', is_production: false })
      loadChannelConfig('wechat')
      loadChannelConfig('alipay')
    }
  }, [open, merchantId])

  const saveWechat = async () => {
    if (!merchantId) return
    const v = await wechatForm.validateFields()
    setSaving(true)
    try {
      await api.put(`/admin/merchants/${merchantId}/channels`, {
        channel: 'wechat',
        config: {
          mch_id: v.mch_id.trim(),
          app_id: v.app_id.trim(),
          api_v3_key: v.api_v3_key.trim(),
          serial_no: v.serial_no.trim(),
          private_key_pem: v.private_key_pem,
          // Optional: new public-key verification path. Fill these when the
          // merchant was registered in 2024+ (WeChat no longer issues
          // platform certificates to new merchants).
          public_key_id: v.public_key_id.trim(),
          public_key_pem: v.public_key_pem,
        },
      })
      message.success('微信渠道已保存')
      loadConfigured()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const saveAlipay = async () => {
    if (!merchantId) return
    const v = await alipayForm.validateFields()
    setSaving(true)
    try {
      await api.put(`/admin/merchants/${merchantId}/channels`, {
        channel: 'alipay',
        config: {
          app_id: v.app_id.trim(),
          private_key: v.private_key,
          alipay_public_key: v.alipay_public_key,
          sign_type: v.sign_type || 'RSA2',
          is_production: !!v.is_production,
        },
      })
      message.success('支付宝渠道已保存')
      loadConfigured()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  // Upload.beforeUpload returning false prevents auto-upload; we just use it
  // as a file picker that feeds the textarea.
  const pemUpload = (formField: string, form: any): UploadProps => ({
    accept: '.pem,.txt,.key',
    showUploadList: false,
    beforeUpload: async (file) => {
      try {
        const text = await readFileText(file)
        form.setFieldValue(formField, text)
        message.success(`已读入 ${file.name}`)
      } catch (e: any) {
        message.error(`读取失败: ${e.message}`)
      }
      return false
    },
  })

  // Batch import: user picks multiple files from their WeChat cert folder,
  // we sniff each one by content (and filename as a fallback) and drop it
  // into the matching form field. Certificate parsing happens on the server
  // because the browser has no built-in X.509 parser.
  const batchUpload: UploadProps = {
    multiple: true,
    showUploadList: false,
    accept: '.pem,.txt,.key,.crt',
    beforeUpload: async (file, fileList) => {
      // Ant Design calls beforeUpload once per file in the batch — process
      // on the last one so we report one summary message.
      const isLast = fileList.indexOf(file) === fileList.length - 1
      if (!(file as any)._epProcessed) {
        (file as any)._epProcessed = true
        try {
          const text = await readFileText(file)
          const applied = await applyImportedFile(file.name, text)
          if (applied) {
            message.success(`${file.name} → ${applied}`)
          } else {
            message.warning(`${file.name} 未识别，已跳过`)
          }
        } catch (e: any) {
          message.error(`${file.name} 读取失败: ${e.message}`)
        }
      }
      if (isLast) {
        // nothing extra for now; per-file toasts are enough
      }
      return false
    },
  }

  // Maps a picked file to a form field, returning a human-readable label for
  // the toast ("商户私钥 → private_key_pem") or an empty string if unmatched.
  async function applyImportedFile(name: string, content: string): Promise<string> {
    const lower = name.toLowerCase()

    // Order matters: check PUBLIC KEY before PRIVATE KEY so "PRIVATE" substring
    // doesn't false-positive on a public key PEM.
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
        // Bonus: subject CN on a wechat merchant cert is the mch_id — if the
        // user hasn't filled it yet we offer that value too.
        if (data.data.subject && /^\d{8,16}$/.test(data.data.subject)) {
          const current = wechatForm.getFieldValue('mch_id')
          if (!current) {
            wechatForm.setFieldValue('mch_id', data.data.subject)
          }
        }
        return `商户证书 → serial_no (${String(data.data.serial_no).slice(0, 12)}…)`
      } catch (e: any) {
        message.error('证书解析失败: ' + (e.response?.data?.msg || e.message))
        return ''
      }
    }

    // APIv3 密钥 .txt file: single line of ~32 ASCII chars, optionally padded
    // with whitespace/newlines.
    if (lower.includes('apiv3') || lower.includes('密钥')) {
      const trimmed = content.trim()
      if (/^[\x20-\x7e]{32}$/.test(trimmed)) {
        wechatForm.setFieldValue('api_v3_key', trimmed)
        return 'APIv3 密钥 → api_v3_key'
      }
    }

    // .p12 bundle — not needed, skip silently
    if (lower.endsWith('.p12')) {
      return '.p12 不需要（已跳过）'
    }

    return ''
  }

  const statusTag = (ch: 'wechat' | 'alipay') => {
    const row = configured[ch]
    if (!row) return <Tag>未配置</Tag>
    return row.status === 1
      ? <Tag color="green">已启用 · 更新于 {row.updated_at?.slice(0, 16).replace('T', ' ')}</Tag>
      : <Tag color="orange">已停用</Tag>
  }

  return (
    <Drawer
      title={
        <div>
          <div style={{
            fontSize: 11,
            letterSpacing: '0.15em',
            color: 'var(--accent-gold)',
            marginBottom: 8,
          }}>
            渠道配置
          </div>
          <div style={{
            fontFamily: 'var(--font-display)',
            fontSize: 30,
            color: 'var(--text-primary)',
            lineHeight: 1,
          }}>
            {merchantName || '商户'}
          </div>
        </div>
      }
      open={open}
      onClose={onClose}
      width={680}
      destroyOnClose
    >
      <Tabs
        activeKey={tab}
        onChange={(k) => setTab(k as 'wechat' | 'alipay')}
        items={[
          {
            key: 'wechat',
            label: <Space>微信支付{statusTag('wechat')}</Space>,
            children: (
              <>
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 16 }}
                  message="全部字段在 商户平台 → 账户中心 → API 安全 里可以找到。微信支付公钥需要先在『验证微信支付身份 → 微信支付公钥』申请并下载。"
                />

                <div
                  style={{
                    marginBottom: 20,
                    padding: 16,
                    border: '1px dashed var(--border-hairline, #444)',
                    borderRadius: 6,
                    background: 'rgba(154, 130, 74, 0.06)',
                  }}
                >
                  <div style={{ marginBottom: 8, fontWeight: 500 }}>
                    📁 批量导入证书文件
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--text-secondary, #888)', marginBottom: 12 }}>
                    一次性选择微信证书文件夹里的 <code>apiclient_cert.pem</code>、<code>apiclient_key.pem</code>、<code>APIV3密钥.txt</code>，以及从商户平台下载的 <strong>微信支付公钥 .pem</strong>，
                    easy-pay 会自动识别类型并填进对应字段（证书里的 serial_no 由服务端 x509 解析）。
                  </div>
                  <Upload {...batchUpload}>
                    <Button icon={<UploadOutlined />}>选择多个文件导入</Button>
                  </Upload>
                </div>

                <Form form={wechatForm} layout="vertical" autoComplete="off">
                  <Form.Item
                    name="mch_id"
                    label="商户号 mch_id"
                    rules={[{ required: true, message: '必填' }, { pattern: /^\d{8,16}$/, message: '应为 8-16 位数字' }]}
                    extra="10 位数字，商户平台『账户中心 → 商户信息』"
                  >
                    <Input placeholder="1900000000" />
                  </Form.Item>

                  <Form.Item
                    name="app_id"
                    label="AppID"
                    rules={[{ required: true, message: '必填' }]}
                    extra="绑定的公众号/小程序/开放平台 AppID，通常以 wx 开头"
                  >
                    <Input placeholder="wx + 16 位字符" />
                  </Form.Item>

                  <Form.Item
                    name="api_v3_key"
                    label="APIv3 密钥"
                    rules={[
                      { required: true, message: '必填' },
                      {
                        validator: (_, v) => {
                          if (!v || isKept(v)) return Promise.resolve()
                          return String(v).length === 32
                            ? Promise.resolve()
                            : Promise.reject(new Error('必须正好 32 个字符'))
                        },
                      },
                    ]}
                    extra="账户中心 → API 安全 → APIv3 密钥 里自己设置的 32 字节字符串（已配置时显示 __KEEP__，不改留着即可）"
                  >
                    <Input.Password placeholder="32 个字符" />
                  </Form.Item>

                  <Form.Item
                    name="serial_no"
                    label="证书序列号 serial_no"
                    rules={[{ required: true, message: '必填' }]}
                    extra="账户中心 → API 安全 → API 证书 列表里的 40 位十六进制序列号"
                  >
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
                      { required: true, message: '必填' },
                      {
                        validator: (_, v) => {
                          if (!v || isKept(v)) return Promise.resolve()
                          if (!String(v).includes('BEGIN') || !String(v).includes('PRIVATE KEY')) {
                            return Promise.reject(new Error('看起来不是一个 PEM 格式的私钥'))
                          }
                          return Promise.resolve()
                        },
                      },
                    ]}
                    extra="把 apiclient_key.pem 文件拖到选择框或手动粘贴，服务端会 AES-256-GCM 加密存储（已配置时显示 __KEEP__，不改留着即可）"
                  >
                    <Input.TextArea rows={10} placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----" style={{ fontFamily: 'monospace', fontSize: 12 }} />
                  </Form.Item>

                  <Form.Item
                    name="public_key_id"
                    label="微信支付公钥 ID (public_key_id)"
                    rules={[
                      { required: true, message: '必填' },
                      { pattern: /^PUB_KEY_ID_/, message: '格式应为 PUB_KEY_ID_开头' },
                    ]}
                    extra="账户中心 → API 安全 → 验证微信支付身份 → 微信支付公钥 点『管理公钥』里能看到，形如 PUB_KEY_ID_0117..."
                  >
                    <Input placeholder="PUB_KEY_ID_0117188168252026041500111613002205" />
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
                      { required: true, message: '必填' },
                      {
                        validator: (_, v) => {
                          if (!v || isKept(v)) return Promise.resolve()
                          if (!String(v).includes('BEGIN') || !String(v).includes('PUBLIC KEY')) {
                            return Promise.reject(new Error('看起来不是一个 PEM 格式的公钥'))
                          }
                          return Promise.resolve()
                        },
                      },
                    ]}
                    extra="从商户平台下载的微信支付公钥文件（一般叫 wxp_pub.pem 或 pub_key.pem），直接选文件即可（已配置时显示 __KEEP__，不改留着即可）"
                  >
                    <Input.TextArea rows={8} placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----" style={{ fontFamily: 'monospace', fontSize: 12 }} />
                  </Form.Item>

                  <Button type="primary" loading={saving} onClick={saveWechat}>
                    保存微信渠道
                  </Button>
                </Form>
              </>
            ),
          },
          {
            key: 'alipay',
            label: <Space>支付宝{statusTag('alipay')}</Space>,
            children: (
              <>
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 16 }}
                  message="支付宝渠道的真实 SDK 还没接入，目前下单会返回占位数据。配置可以先填好，等接入后即用。"
                />
                <Form form={alipayForm} layout="vertical" autoComplete="off">
                  <Form.Item name="app_id" label="AppID" rules={[{ required: true }]}
                    extra="支付宝开放平台的应用 AppID">
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
                    extra="开放平台 → 应用 → 开发设置 → 接口加签方式 生成的应用私钥"
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
                    extra="开放平台 → 应用 → 开发设置 里保存应用公钥后支付宝返回的公钥"
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
                    保存支付宝渠道
                  </Button>
                </Form>
              </>
            ),
          },
        ]}
      />
    </Drawer>
  )
}
