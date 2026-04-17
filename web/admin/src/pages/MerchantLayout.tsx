import { BellOutlined, FileTextOutlined, LogoutOutlined, MenuOutlined, SettingOutlined, ShopOutlined, CloseOutlined } from '@ant-design/icons'
import { Layout as AntLayout, Menu } from 'antd'
import { useEffect, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { MERCHANT_TOKEN_KEY, merchantApi } from '../api'

const { Sider, Content } = AntLayout

const pageTitles: Record<string, { crumb: string; section: string }> = {
  '/merchant/orders': { crumb: '我的账单', section: '交易' },
  '/merchant/notify-logs': { crumb: '回调记录', section: '监控' },
  '/merchant/settings': { crumb: '商户设置', section: '资料' },
}

function formatNow() {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}.${pad(d.getMonth() + 1)}.${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export default function MerchantLayout() {
  const nav = useNavigate()
  const loc = useLocation()
  const [now, setNow] = useState<string>(() => formatNow())
  const [navOpen, setNavOpen] = useState(false)

  useEffect(() => {
    const id = setInterval(() => setNow(formatNow()), 1000)
    return () => clearInterval(id)
  }, [])

  useEffect(() => {
    setNavOpen(false)
  }, [loc.pathname])

  useEffect(() => {
    document.body.classList.toggle('ep-nav-locked', navOpen)
    return () => document.body.classList.remove('ep-nav-locked')
  }, [navOpen])

  const logout = async () => {
    try { await merchantApi.post('/merchant/logout') } catch {}
    localStorage.removeItem(MERCHANT_TOKEN_KEY)
    nav('/merchant-login')
  }

  const items = [
    { key: '/merchant/orders', icon: <FileTextOutlined />, label: '我的账单' },
    { key: '/merchant/notify-logs', icon: <BellOutlined />, label: '回调记录' },
    { key: '/merchant/settings', icon: <SettingOutlined />, label: '商户设置' },
  ]

  const current = pageTitles[loc.pathname] ?? { crumb: '商户中心', section: '控制台' }

  return (
    <AntLayout className={`ep-shell ep-shell--merchant${navOpen ? ' ep-shell--nav-open' : ''}`} style={{ minHeight: '100vh', background: 'transparent' }}>
      <div className="ep-nav-backdrop" aria-hidden={!navOpen} onClick={() => setNavOpen(false)} />

      <Sider className="ep-sider" width={260}>
        <div className="ep-brand">
          <div className="ep-brand-mark">mc</div>
          <div className="ep-brand-text">
            <span className="wordmark">商户中心</span>
            <span className="caption">易支付 · self-service</span>
          </div>
          <button type="button" className="ep-sider-close" aria-label="关闭菜单" onClick={() => setNavOpen(false)}>
            <CloseOutlined />
          </button>
        </div>

        <div className="ep-section-label">导航</div>
        <Menu mode="inline" selectedKeys={[loc.pathname]} onClick={(e) => nav(e.key)} items={items} />

        <div className="ep-sider-footer">
          <div className="row"><ShopOutlined style={{ marginRight: 10 }} />当前视图仅限本人商户</div>
          <div className="row" style={{ marginTop: 6, color: 'var(--text-faint)' }}>资料、账单与通知记录</div>
        </div>
      </Sider>

      <AntLayout style={{ background: 'transparent' }}>
        <div className="ep-header">
          <button type="button" className="ep-nav-trigger" aria-label="打开菜单" aria-expanded={navOpen} onClick={() => setNavOpen(true)}>
            <MenuOutlined />
          </button>
          <div className="ep-breadcrumb">
            <span className="crumb-root">商户中心</span>
            <span className="divider">/</span>
            <span className="crumb-section">{current.section}</span>
            <span className="divider">/</span>
            <span className="current">{current.crumb}</span>
          </div>
          <div className="ep-header-right">
            <div className="ep-time-ticker">
              <span className="label">北京时间</span>
              <span>{now}</span>
            </div>
            <button className="ep-ghost-btn" onClick={logout}>
              <LogoutOutlined /> <span className="ep-ghost-btn-label">退出登录</span>
            </button>
          </div>
        </div>
        <Content className="ep-content">
          <Outlet />
        </Content>
      </AntLayout>
    </AntLayout>
  )
}
