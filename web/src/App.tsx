import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import axios from 'axios'
import { TOKEN_KEY, getRole } from './api'
import Layout from './pages/Layout'
import Login from './pages/Login'
import MerchantNotifyLogs from './pages/MerchantNotifyLogs'
import MerchantOrders from './pages/MerchantOrders'
import MerchantSettings from './pages/MerchantSettings'
import Merchants from './pages/Merchants'
import NotifyLogs from './pages/NotifyLogs'
import Orders from './pages/Orders'
import PlatformChannels from './pages/PlatformChannels'
import Settings from './pages/Settings'
import Setup from './pages/Setup'
import IntegrationDoc from './pages/IntegrationDoc'
import Settlements from './pages/Settlements'
import MerchantSettlements from './pages/MerchantSettlements'
import AdminDashboard from './pages/AdminDashboard'
import MerchantDashboard from './pages/MerchantDashboard'

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

        <Route
          path="/"
          element={
            <AuthGuard>
              <Layout />
            </AuthGuard>
          }
        >
          {/* Default redirect based on role */}
          <Route index element={<RoleRedirect />} />

          {/* Admin management */}
          <Route path="dashboard" element={<AdminDashboard />} />
          <Route path="merchants" element={<Merchants />} />
          <Route path="orders" element={<Orders />} />
          <Route path="notify-logs" element={<NotifyLogs />} />
          <Route path="platform" element={<PlatformChannels />} />
          <Route path="settlements" element={<Settlements />} />
          <Route path="settings" element={<Settings />} />

          {/* Personal (all roles) */}
          <Route path="my-dashboard" element={<MerchantDashboard />} />
          <Route path="my-orders" element={<MerchantOrders />} />
          <Route path="my-notify-logs" element={<MerchantNotifyLogs />} />
          <Route path="my-settings" element={<MerchantSettings />} />
          <Route path="my-settlements" element={<MerchantSettlements />} />

          {/* Docs */}
          <Route path="docs/integration" element={<IntegrationDoc />} />
        </Route>
      </Routes>
    </SetupGate>
  )
}

function RoleRedirect() {
  const role = getRole()
  return <Navigate to={role === 'admin' ? '/dashboard' : '/my-dashboard'} replace />
}

export default function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  )
}
