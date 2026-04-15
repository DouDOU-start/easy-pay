import { Layout as AntLayout, Menu } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { ShopOutlined, FileTextOutlined, BellOutlined, LogoutOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { api } from '../api'

const { Sider, Content } = AntLayout

const pageTitles: Record<string, { crumb: string; section: string }> = {
  '/merchants': { crumb: '商户管理', section: '商户' },
  '/orders': { crumb: '订单中心', section: '交易' },
  '/notify-logs': { crumb: '通知日志', section: '监控' },
}

function formatNow() {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}.${pad(d.getMonth() + 1)}.${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export default function Layout() {
  const nav = useNavigate()
  const loc = useLocation()
  const [now, setNow] = useState<string>(() => formatNow())

  useEffect(() => {
    const id = setInterval(() => setNow(formatNow()), 1000)
    return () => clearInterval(id)
  }, [])

  const logout = async () => {
    try { await api.post('/admin/logout') } catch {}
    localStorage.removeItem('easypay_token')
    nav('/login')
  }

  const items = [
    { key: '/merchants', icon: <ShopOutlined />, label: '商户管理' },
    { key: '/orders', icon: <FileTextOutlined />, label: '订单中心' },
    { key: '/notify-logs', icon: <BellOutlined />, label: '通知日志' },
  ]

  const current = pageTitles[loc.pathname] ?? { crumb: '概览', section: '控制台' }

  return (
    <AntLayout style={{ minHeight: '100vh', background: 'transparent' }}>
      <Sider className="ep-sider" width={260}>
        <div className="ep-brand">
          <div className="ep-brand-mark">ep</div>
          <div className="ep-brand-text">
            <span className="wordmark">易支付</span>
            <span className="caption">支付管理平台 · v0.1</span>
          </div>
        </div>

        <div className="ep-section-label">导航</div>
        <Menu
          mode="inline"
          selectedKeys={[loc.pathname]}
          onClick={(e) => nav(e.key)}
          items={items}
        />

        <div className="ep-sider-footer">
          <div className="row"><span className="status-dot" />网关 · 运行正常</div>
          <div className="row" style={{ marginTop: 6, color: 'var(--text-faint)' }}>
            © 2026 · 安全通道
          </div>
        </div>
      </Sider>

      <AntLayout style={{ background: 'transparent' }}>
        <div className="ep-header">
          <div className="ep-breadcrumb">
            <span>易支付</span>
            <span className="divider">/</span>
            <span>{current.section}</span>
            <span className="divider">/</span>
            <span className="current">{current.crumb}</span>
          </div>
          <div className="ep-header-right">
            <div className="ep-time-ticker">
              <span className="label">北京时间</span>
              <span>{now}</span>
            </div>
            <button className="ep-ghost-btn" onClick={logout}>
              <LogoutOutlined /> 退出登录
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
