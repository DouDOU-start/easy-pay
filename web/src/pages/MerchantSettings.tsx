import { useEffect, useState } from 'react'
import { Button, Form, Input, message } from 'antd'
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

export default function MerchantSettings() {
  const [loading, setLoading] = useState(false)
  const [passwordLoading, setPasswordLoading] = useState(false)
  const [profile, setProfile] = useState<MerchantProfile | null>(null)
  const [profileForm] = Form.useForm()
  const [passwordForm] = Form.useForm()

  const load = async () => {
    const { data } = await merchantApi.get('/merchant/me')
    const next = data.data as MerchantProfile
    setProfile(next)
    profileForm.setFieldsValue({
      name: next.name,
      notify_url: next.notify_url,
      email: next.email,
      mch_no: next.mch_no,
      app_id: next.app_id,
    })
  }

  useEffect(() => { load() }, [])

  const saveProfile = async () => {
    const v = await profileForm.validateFields()
    setLoading(true)
    try {
      await merchantApi.put('/merchant/me', {
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
      await merchantApi.put('/merchant/me/password', {
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

  return (
    <>
      <div className="ep-page-header">
        <div className="col-title">
          <div className="eyebrow">Profile</div>
          <h1>商户资料支持直接修改<em>名称</em>。</h1>
          <div className="subtitle">这里维护商户展示名称、通知地址和登录密码。邮箱、商户号与应用 ID 由平台分配，不在商户端开放修改。</div>
        </div>
      </div>

      <div className="ep-settings-grid">
        <section className="ep-panel ep-settings-panel">
          <div className="ep-settings-head">
            <div>
              <div className="ep-settings-kicker">资料</div>
              <h3>基础信息</h3>
            </div>
            <div className="ep-settings-hint">当前状态：{profile?.status === 1 ? '启用' : '停用'}</div>
          </div>

          <Form form={profileForm} layout="vertical" requiredMark={false}>
            <div className="ep-form-grid ep-form-grid--2">
              <Form.Item name="mch_no" label="商户号">
                <Input disabled />
              </Form.Item>
              <Form.Item name="app_id" label="应用 ID">
                <Input disabled />
              </Form.Item>
            </div>
            <div className="ep-form-grid ep-form-grid--2">
              <Form.Item name="email" label="登录邮箱">
                <Input disabled />
              </Form.Item>
              <Form.Item
                name="name"
                label="商户名称"
                rules={[{ required: true, message: '请输入商户名称' }]}
                extra="这里的名称可以修改，会同步显示到后台商户列表和订单页。"
              >
                <Input placeholder="请输入商户展示名称" />
              </Form.Item>
            </div>
            <Form.Item name="notify_url" label="下游 Notify URL" extra="留空表示暂不配置回调地址。">
              <Input placeholder="https://your-service/callback" />
            </Form.Item>
            <div className="ep-panel-actions">
              <Button type="primary" onClick={saveProfile} loading={loading}>保存资料</Button>
            </div>
          </Form>
        </section>

        <section className="ep-panel ep-settings-panel">
          <div className="ep-settings-head">
            <div>
              <div className="ep-settings-kicker">Security</div>
              <h3>修改密码</h3>
            </div>
            <div className="ep-settings-hint">建议首次登录后立即更换初始密码</div>
          </div>

          <Form form={passwordForm} layout="vertical" requiredMark={false}>
            <Form.Item name="old_password" label="旧密码" rules={[{ required: true, message: '请输入旧密码' }]}>
              <Input.Password placeholder="请输入当前密码" autoComplete="current-password" />
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
              <Input.Password placeholder="请输入新密码" autoComplete="new-password" />
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
              <Input.Password placeholder="请再次输入新密码" autoComplete="new-password" />
            </Form.Item>
            <div className="ep-panel-actions">
              <Button type="primary" onClick={changePassword} loading={passwordLoading}>更新密码</Button>
            </div>
          </Form>
        </section>
      </div>
    </>
  )
}
