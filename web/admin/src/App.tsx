import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import axios from 'axios'
import { ADMIN_TOKEN_KEY, MERCHANT_TOKEN_KEY } from './api'
import Layout from './pages/Layout'
import Login from './pages/Login'
import MerchantLayout from './pages/MerchantLayout'
import MerchantLogin from './pages/MerchantLogin'
import MerchantNotifyLogs from './pages/MerchantNotifyLogs'
import MerchantOrders from './pages/MerchantOrders'
import MerchantSettings from './pages/MerchantSettings'
import Merchants from './pages/Merchants'
import NotifyLogs from './pages/NotifyLogs'
import Orders from './pages/Orders'
import PlatformChannels from './pages/PlatformChannels'
import Setup from './pages/Setup'
import TestNotify from './pages/TestNotify'

const AdminGuard = ({ children }: { children: JSX.Element }) => {
  const token = localStorage.getItem(ADMIN_TOKEN_KEY)
  return token ? children : <Navigate to="/login" replace />
}

const MerchantGuard = ({ children }: { children: JSX.Element }) => {
  const token = localStorage.getItem(MERCHANT_TOKEN_KEY)
  return token ? children : <Navigate to="/merchant-login" replace />
}

// Checks /setup/status on mount. If setup is required, redirects to /setup.
// Wraps the entire app so every entry point (login, dashboard, etc.) is covered.
function SetupGate({ children }: { children: JSX.Element }) {
  const [checking, setChecking] = useState(true)
  const nav = useNavigate()

  useEffect(() => {
    // Skip the check if we're already on /setup
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
      .catch(() => { /* ignore — server may be restarting */ })
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
        <Route path="/merchant-login" element={<MerchantLogin />} />

        <Route
          path="/"
          element={
            <AdminGuard>
              <Layout />
            </AdminGuard>
          }
        >
          <Route index element={<Navigate to="/merchants" replace />} />
          <Route path="merchants" element={<Merchants />} />
          <Route path="orders" element={<Orders />} />
          <Route path="notify-logs" element={<NotifyLogs />} />
          <Route path="platform" element={<PlatformChannels />} />
          <Route path="test-notify" element={<TestNotify />} />
        </Route>

        <Route
          path="/merchant"
          element={
            <MerchantGuard>
              <MerchantLayout />
            </MerchantGuard>
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
