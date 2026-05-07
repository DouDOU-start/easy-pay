import { Layout as AntLayout, Menu } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  ShopOutlined,
  FileTextOutlined,
  BellOutlined,
  LogoutOutlined,
  ApiOutlined,
  SettingOutlined,
  UserOutlined,
  MenuOutlined,
  CloseOutlined,
  BookOutlined,
} from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { authApi, clearSession, getRole, fetchVersion } from '../api'

const { Sider, Content } = AntLayout

const pageTitles: Record<string, { crumb: string; section: string }> = {
  '/merchants': { crumb: '商户管理', section: '管理' },
  '/orders': { crumb: '订单中心', section: '管理' },
  '/notify-logs': { crumb: '通知日志', section: '管理' },
  '/platform': { crumb: '渠道凭证', section: '管理' },
  '/settings': { crumb: '系统设置', section: '管理' },
  '/my-orders': { crumb: '我的订单', section: '个人' },
  '/my-notify-logs': { crumb: '我的通知', section: '个人' },
  '/my-settings': { crumb: '账户设置', section: '个人' },
  '/docs/integration': { crumb: '对接文档', section: '文档' },
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
  const [navOpen, setNavOpen] = useState(false)
  const [ver, setVer] = useState('')

  useEffect(() => { fetchVersion().then(setVer).catch(() => {}) }, [])

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
    try { await authApi.logout() } catch {}
    clearSession()
    nav('/login')
  }

  const isAdmin = getRole() === 'admin'

  const personalItems = [
    { key: '/my-orders', icon: <FileTextOutlined />, label: '我的订单' },
    { key: '/my-notify-logs', icon: <BellOutlined />, label: '我的通知' },
    { key: '/my-settings', icon: <UserOutlined />, label: '账户设置' },
  ]

  const docItems = [
    { key: '/docs/integration', icon: <BookOutlined />, label: '对接文档' },
  ]

  const items = isAdmin
    ? [
        { type: 'group' as const, label: '管理', children: [
          { key: '/merchants', icon: <ShopOutlined />, label: '商户管理' },
          { key: '/orders', icon: <FileTextOutlined />, label: '订单中心' },
          { key: '/notify-logs', icon: <BellOutlined />, label: '通知日志' },
          { key: '/platform', icon: <ApiOutlined />, label: '渠道凭证' },
          { key: '/settings', icon: <SettingOutlined />, label: '系统设置' },
        ]},
        { type: 'group' as const, label: '个人', children: personalItems },
        { type: 'group' as const, label: '文档', children: docItems },
      ]
    : [
        ...personalItems,
        { type: 'group' as const, label: '文档', children: docItems },
      ]

  const current = pageTitles[loc.pathname] ?? { crumb: '概览', section: '控制台' }

  return (
    <AntLayout className={`ep-shell${navOpen ? ' ep-shell--nav-open' : ''}`} style={{ minHeight: '100vh', background: 'transparent' }}>
      <div
        className="ep-nav-backdrop"
        aria-hidden={!navOpen}
        onClick={() => setNavOpen(false)}
      />

      <Sider className="ep-sider" width={260}>
        <div className="ep-brand">
          <div className="ep-brand-mark">{isAdmin ? 'ep' : 'mc'}</div>
          <div className="ep-brand-text">
            <span className="wordmark">{isAdmin ? '易支付' : '商户中心'}</span>
            <span className="caption">{isAdmin ? `支付管理平台${ver ? ' · ' + ver : ''}` : '易支付 · self-service'}</span>
          </div>
          <button
            type="button"
            className="ep-sider-close"
            aria-label="关闭菜单"
            onClick={() => setNavOpen(false)}
          >
            <CloseOutlined />
          </button>
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

      <AntLayout className="ep-main" style={{ background: 'transparent' }}>
        <div className="ep-header">
          <button
            type="button"
            className="ep-nav-trigger"
            aria-label="打开菜单"
            aria-expanded={navOpen}
            onClick={() => setNavOpen(true)}
          >
            <MenuOutlined />
          </button>
          <div className="ep-breadcrumb">
            <span className="crumb-root">易支付</span>
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
