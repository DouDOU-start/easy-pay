import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Login from './pages/Login'
import Layout from './pages/Layout'
import Merchants from './pages/Merchants'
import Orders from './pages/Orders'
import NotifyLogs from './pages/NotifyLogs'

const Guard = ({ children }: { children: JSX.Element }) => {
  const token = localStorage.getItem('easypay_token')
  return token ? children : <Navigate to="/login" replace />
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="/"
          element={
            <Guard>
              <Layout />
            </Guard>
          }
        >
          <Route index element={<Navigate to="/merchants" replace />} />
          <Route path="merchants" element={<Merchants />} />
          <Route path="orders" element={<Orders />} />
          <Route path="notify-logs" element={<NotifyLogs />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
