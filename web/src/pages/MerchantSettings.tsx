import { useEffect, useState } from 'react'
import { Button, Form, Input, Modal, message, Tag } from 'antd'
import { CopyOutlined, ExclamationCircleOutlined } from '@ant-design/icons'
import { merchantApi } from '../api'

interface MerchantProfile {
  id: number
  mch_no: string
  name: string
  email: string
  notify_url: string
  app_id: string
  status: number
  created_at: string
}

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => message.success('已复制'))
}

function ReadonlyField({ label, value }: { label: string; value?: string }) {
  return (
    <div className="ep-readonly-field">
      <span className="ep-readonly-label">{label}</span>
      <span className="ep-readonly-value">
        <span>{value || '-'}</span>
        {value && (
          <CopyOutlined className="ep-readonly-copy" onClick={() => copyText(value)} />
        )}
      </span>
    </div>
  )
}

export default function MerchantSettings() {
  const [loading, setLoading] = useState(false)
  const [passwordLoading, setPasswordLoading] = useState(false)
  const [profile, setProfile] = useState<MerchantProfile | null>(null)
  const [secretOpen, setSecretOpen] = useState(false)
  const [secret, setSecret] = useState<{ app_id: string; app_secret: string } | null>(null)
  const [profileForm] = Form.useForm()
  const [passwordForm] = Form.useForm()

  const load = async () => {
    const next = await merchantApi.me() as MerchantProfile
    setProfile(next)
    profileForm.setFieldsValue({
      name: next.name,
      notify_url: next.notify_url,
    })
  }

  useEffect(() => { load() }, [])

  const saveProfile = async () => {
    const v = await profileForm.validateFields()
    setLoading(true)
    try {
      await merchantApi.updateProfile({
        name: v.name,
        notify_url: v.notify_url ?? '',
      })
      message.success('商户资料已更新')
      await load()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '保存失败')
    } finally {
      setLoading(false)
    }
  }

  const changePassword = async () => {
    const v = await passwordForm.validateFields()
    setPasswordLoading(true)
    try {
      await merchantApi.changePassword({
        old_password: v.old_password,
        new_password: v.new_password,
      })
      message.success('密码已更新')
      passwordForm.resetFields()
    } catch (e: any) {
      message.error(e.response?.data?.msg || '修改密码失败')
    } finally {
      setPasswordLoading(false)
    }
  }

  const openSecret = async () => {
    try {
      const res = await merchantApi.getSecret()
      setSecret(res)
      setSecretOpen(true)
    } catch (e: any) {
      message.error(e.response?.data?.msg || '获取失败')
    }
  }

  const doResetSecret = () => {
    Modal.confirm({
      title: '重置应用密钥',
      icon: <ExclamationCircleOutlined />,
      content: '重置后旧密钥立即失效，所有使用旧密钥的 API 调用将鉴权失败。确认重置？',
      okText: '确认重置',
      okButtonProps: { danger: true },
      cancelText: '取消',
      centered: true,
      onOk: async () => {
        try {
          const res = await merchantApi.resetSecret()
          setSecret(res)
          message.success('密钥已重置')
        } catch (e: any) {
          message.error(e.response?.data?.msg || '重置失败')
        }
      },
    })
  }

  return (
    <div style={{ width: '100%', maxWidth: 820, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 18, paddingBottom: 40 }}>
      <section className="ep-panel ep-settings-panel">
        <div className="ep-settings-head">
          <h3>基础信息</h3>
          <Tag color={profile?.status === 1 ? 'success' : 'default'}>
            {profile?.status === 1 ? '启用' : '停用'}
          </Tag>
        </div>
        <div className="ep-readonly-list">
          <ReadonlyField label="商户号" value={profile?.mch_no} />
          <ReadonlyField label="应用 ID" value={profile?.app_id} />
          <ReadonlyField label="登录邮箱" value={profile?.email} />
        </div>
      </section>

      <section className="ep-panel ep-settings-panel">
        <div className="ep-settings-head">
          <h3>API 签名凭证</h3>
          <Button size="small" onClick={openSecret}>查看密钥</Button>
        </div>
        <div className="ep-readonly-list">
          <ReadonlyField label="App ID" value={profile?.app_id} />
        </div>
        <div style={{ marginTop: 8, fontSize: 12, color: 'var(--text-faint)' }}>
          点击「查看密钥」获取 API 对接所需的 app_id 和 app_secret
        </div>
      </section>

      <Modal
        title="API 签名凭证"
        open={secretOpen}
        onCancel={() => setSecretOpen(false)}
        centered
        width={520}
        footer={[
          <Button key="reset" danger onClick={doResetSecret}>重置密钥</Button>,
          <Button key="close" type="primary" onClick={() => setSecretOpen(false)}>关闭</Button>,
        ]}
      >
        {secret && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ padding: '14px 16px', background: 'var(--bg-deep)', borderRadius: 6 }}>
              <div style={{ fontSize: 11, color: 'var(--text-tertiary)', marginBottom: 8, letterSpacing: '0.05em' }}>APP ID</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span className="mono" style={{ fontSize: 14, color: 'var(--text-primary)', flex: 1 }}>{secret.app_id}</span>
                <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyText(secret.app_id)} />
              </div>
            </div>
            <div style={{ padding: '14px 16px', background: 'var(--bg-deep)', borderRadius: 6 }}>
              <div style={{ fontSize: 11, color: 'var(--text-tertiary)', marginBottom: 8, letterSpacing: '0.05em' }}>APP SECRET</div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                <span className="mono" style={{ fontSize: 13, color: 'var(--accent-gold)', flex: 1, wordBreak: 'break-all', lineHeight: 1.6 }}>{secret.app_secret}</span>
                <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyText(secret.app_secret)} style={{ flexShrink: 0 }} />
              </div>
            </div>
            <div style={{ fontSize: 11, color: 'var(--text-faint)', padding: '10px 14px', background: 'var(--bg-elevated)', borderRadius: 4, lineHeight: 1.6 }}>
              请妥善保管密钥，勿泄露给第三方。重置后旧密钥立即失效，所有使用旧密钥的 API 调用将鉴权失败。
            </div>
          </div>
        )}
      </Modal>

      <section className="ep-panel ep-settings-panel">
        <div className="ep-settings-head">
          <h3>商户资料</h3>
        </div>
        <Form form={profileForm} layout="vertical" requiredMark={false}>
          <Form.Item
            name="name"
            label="商户名称"
            rules={[{ required: true, message: '请输入商户名称' }]}
          >
            <Input placeholder="请输入商户展示名称" />
          </Form.Item>
          <Form.Item name="notify_url" label="通知地址">
            <Input placeholder="https://your-service/callback" />
          </Form.Item>
          <div className="ep-panel-actions">
            <Button type="primary" onClick={saveProfile} loading={loading}>保存</Button>
          </div>
        </Form>
      </section>

      <section className="ep-panel ep-settings-panel">
        <div className="ep-settings-head">
          <h3>修改密码</h3>
        </div>
        <Form form={passwordForm} layout="vertical" requiredMark={false}>
          <Form.Item name="old_password" label="旧密码" rules={[{ required: true, message: '请输入旧密码' }]}>
            <Input.Password placeholder="当前密码" autoComplete="nope" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '密码至少 8 位' },
              { max: 72, message: '密码不能超过 72 位' },
            ]}
          >
            <Input.Password placeholder="至少 8 位" autoComplete="nope" />
          </Form.Item>
          <Form.Item
            name="confirm_password"
            label="确认新密码"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password placeholder="再次输入" autoComplete="nope" />
          </Form.Item>
          <div className="ep-panel-actions">
            <Button type="primary" onClick={changePassword} loading={passwordLoading}>更新密码</Button>
          </div>
        </Form>
      </section>
    </div>
  )
}
