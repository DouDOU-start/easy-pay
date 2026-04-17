import { Form, Input, Button, message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { ADMIN_TOKEN_KEY, api } from '../api'

export default function Login() {
  const nav = useNavigate()
  const onFinish = async (v: { username: string; password: string }) => {
    try {
      const { data } = await api.post('/admin/login', v)
      localStorage.setItem(ADMIN_TOKEN_KEY, data.data.token)
      message.success('登录成功')
      nav('/merchants')
    } catch (e: any) {
      message.error(e.response?.data?.msg || '登录失败')
    }
  }

  return (
    <div className="ep-login">
      <div className="ep-login-hero">
        <div className="ep-login-brand">
          <div className="mark">ep</div>
          <div>
            <div className="label">易支付</div>
            <div className="caption">支付管理平台 · v0.1</div>
          </div>
        </div>

        <div className="ep-login-headline">
          <div className="eyebrow">安全 · 支付管理控制台</div>
          <h1>
            让每一笔支付，<br />
            <em>举重若轻</em>。
          </h1>
          <p>
            统一接入微信、支付宝等主流收单渠道，在同一个面板里管理商户、追踪订单、复核回调通知。
            低延迟、高可观测性的现代化支付网关。
          </p>
        </div>

        <div className="ep-login-stats">
          <div className="stat">
            <span className="label">网关可用性</span>
            <span className="value">99.998<span className="unit">%</span></span>
          </div>
          <div className="stat">
            <span className="label">已接入渠道</span>
            <span className="value">02<span className="unit">/ 04</span></span>
          </div>
          <div className="stat">
            <span className="label">平均延迟</span>
            <span className="value">128<span className="unit">ms</span></span>
          </div>
        </div>
      </div>

      <div className="ep-login-form-wrap">
        <div className="ep-login-form">
          <div className="form-eyebrow">身份认证</div>
          <h2>
            欢迎<em>回来</em>。
          </h2>
          <p className="sub">登录以进入商户管理台。</p>

          <Form
            layout="vertical"
            onFinish={onFinish}
            requiredMark={false}
          >
            <Form.Item name="username" label="邮箱" rules={[{ required: true, message: '请输入邮箱' }]}>
              <Input placeholder="admin@example.com" autoComplete="username" />
            </Form.Item>
            <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password placeholder="••••••••" autoComplete="current-password" />
            </Form.Item>
            <Button type="primary" htmlType="submit" block>
              登 录
            </Button>
          </Form>

          <div className="ep-login-footer">
            <span><span className="dot" />TLS 1.3 加密传输</span>
            <span>易支付 · 管理台</span>
          </div>
        </div>
      </div>
    </div>
  )
}
