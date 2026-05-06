import { Button, Form, Input, message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { MERCHANT_TOKEN_KEY, merchantApi } from '../api'

export default function MerchantLogin() {
  const nav = useNavigate()

  const onFinish = async (v: { email: string; password: string }) => {
    try {
      const { data } = await merchantApi.post('/merchant/login', v)
      localStorage.setItem(MERCHANT_TOKEN_KEY, data.data.token)
      message.success('登录成功')
      nav('/merchant/orders')
    } catch (e: any) {
      message.error(e.response?.data?.msg || '登录失败')
    }
  }

  return (
    <div className="ep-login ep-login--merchant">
      <div className="ep-login-hero">
        <div className="ep-login-brand">
          <div className="mark">mp</div>
          <div>
            <div className="label">易支付商户中心</div>
            <div className="caption">Merchant Console · v0.1</div>
          </div>
        </div>

        <div className="ep-login-headline">
          <div className="eyebrow">账单 · 回调 · 资料维护</div>
          <h1>
            管好自己的订单，<br />
            <em>也管好自己的资料</em>。
          </h1>
          <p>
            商户可直接查看账单、核对回调结果，并维护商户名称、通知地址与登录密码。
            名称修改会实时反映在管理台与交易列表中。
          </p>
        </div>

        <div className="ep-login-stats">
          <div className="stat">
            <span className="label">查询范围</span>
            <span className="value">100<span className="unit">%</span></span>
          </div>
          <div className="stat">
            <span className="label">访问隔离</span>
            <span className="value">01<span className="unit"> 商户</span></span>
          </div>
          <div className="stat">
            <span className="label">资料更新</span>
            <span className="value">实时<span className="unit"> 生效</span></span>
          </div>
        </div>
      </div>

      <div className="ep-login-form-wrap">
        <div className="ep-login-form">
          <div className="form-eyebrow">商户认证</div>
          <h2>
            欢迎进入<em>商户中心</em>。
          </h2>
          <p className="sub">使用管理员分配的邮箱和密码登录。</p>

          <Form layout="vertical" onFinish={onFinish} requiredMark={false}>
            <Form.Item
              name="email"
              label="邮箱"
              rules={[
                { required: true, message: '请输入邮箱' },
                { type: 'email', message: '请输入有效邮箱' },
              ]}
            >
              <Input placeholder="merchant@example.com" autoComplete="username" />
            </Form.Item>
            <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password placeholder="••••••••" autoComplete="current-password" />
            </Form.Item>
            <Button type="primary" htmlType="submit" block>
              登 录 商 户 中 心
            </Button>
          </Form>

          <div className="ep-login-footer">
            <span><span className="dot" />仅可访问当前商户数据</span>
            <span>易支付 · 商户中心</span>
          </div>
        </div>
      </div>
    </div>
  )
}
