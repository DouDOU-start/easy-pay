import { useEffect, useState } from 'react'
import { Form, Input, Button, message, Alert } from 'antd'
import { adminApi } from '../api'

export default function Settings() {
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    adminApi.getSettings()
      .then((settings) => {
        form.setFieldsValue({ platform_base: settings?.platform_base ?? '' })
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [form])

  const onSave = async () => {
    const vals = await form.validateFields()
    setSaving(true)
    try {
      await adminApi.updateSettings({ platform_base: vals.platform_base })
      message.success('设置已保存')
    } catch (e: any) {
      message.error(e.response?.data?.msg ?? '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 24 }}
        message="回调基础地址用于生成支付渠道的异步通知 URL，格式如 https://pay.example.com（不含末尾斜杠）。修改后立即对新订单生效。"
      />
      <Form form={form} layout="vertical" style={{ maxWidth: 600 }}>
        <Form.Item
          name="platform_base"
          label="回调基础地址"
          rules={[{ type: 'url', message: '请输入有效的 URL' }]}
          extra="例如: https://pay.example.com，留空则使用默认地址"
        >
          <Input placeholder="https://pay.example.com" disabled={loading} />
        </Form.Item>
        <Button type="primary" loading={saving} onClick={onSave}>
          保存设置
        </Button>
      </Form>
    </>
  )
}
