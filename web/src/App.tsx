import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import axios from 'axios'
import { TOKEN_KEY, getRole } from './api'
import Layout from './pages/Layout'
import Login from './pages/Login'
import MerchantLayout from './pages/MerchantLayout'
import MerchantNotifyLogs from './pages/MerchantNotifyLogs'
import MerchantOrders from './pages/MerchantOrders'
import MerchantSettings from './pages/MerchantSettings'
import Merchants from './pages/Merchants'
import NotifyLogs from './pages/NotifyLogs'
import Orders from './pages/Orders'
import PlatformChannels from './pages/PlatformChannels'
import Settings from './pages/Settings'
import Setup from './pages/Setup'

const AuthGuard = ({ children, requiredRole }: { children: JSX.Element; requiredRole?: string }) => {
  const token = localStorage.getItem(TOKEN_KEY)
  if (!token) return <Navigate to="/login" replace />
  if (requiredRole) {
    const role = getRole()
    if (role !== requiredRole) return <Navigate to="/login" replace />
  }
  return children
}

function SetupGate({ children }: { children: JSX.Element }) {
  const [checking, setChecking] = useState(true)
  const nav = useNavigate()

  useEffect(() => {
    if (window.location.pathname === '/setup') {
      setChecking(false)
      return
    }
    axios.get('/setup/status')
      .then((res) => {
        if (res.data?.setup_required) {
          nav('/setup', { replace: true })
        }
      })
      .catch(() => {})
      .finally(() => setChecking(false))
  }, [nav])

  if (checking) return null
  return children
}

function AppRoutes() {
  return (
    <SetupGate>
      <Routes>
        <Route path="/setup" element={<Setup />} />
        <Route path="/login" element={<Login />} />

        {/* Admin panel */}
        <Route
          path="/"
          element={
            <AuthGuard requiredRole="admin">
              <Layout />
            </AuthGuard>
          }
        >
          <Route index element={<Navigate to="/merchants" replace />} />
          <Route path="merchants" element={<Merchants />} />
          <Route path="orders" element={<Orders />} />
          <Route path="notify-logs" element={<NotifyLogs />} />
          <Route path="platform" element={<PlatformChannels />} />
          <Route path="settings" element={<Settings />} />
        </Route>

        {/* Merchant self-service (accessible to all roles) */}
        <Route
          path="/merchant"
          element={
            <AuthGuard>
              <MerchantLayout />
            </AuthGuard>
          }
        >
          <Route index element={<Navigate to="/merchant/orders" replace />} />
          <Route path="orders" element={<MerchantOrders />} />
          <Route path="notify-logs" element={<MerchantNotifyLogs />} />
          <Route path="settings" element={<MerchantSettings />} />
        </Route>
      </Routes>
    </SetupGate>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  )
}
