import axios from 'axios'

// --------------- axios instance ---------------

export const TOKEN_KEY = 'easypay_token'

export const http = axios.create({ baseURL: '/' })

http.interceptors.request.use((cfg) => {
  const t = localStorage.getItem(TOKEN_KEY)
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

http.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY)
      if (location.pathname !== '/login') location.href = '/login'
    }
    return Promise.reject(err)
  },
)

// --------------- session helpers ---------------

export function getRole(): string | null {
  try {
    const raw = localStorage.getItem('easypay_user')
    if (!raw) return null
    return JSON.parse(raw).role ?? null
  } catch {
    return null
  }
}

export function setUser(user: { id: number; name: string; email: string; role: string }) {
  localStorage.setItem('easypay_user', JSON.stringify(user))
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem('easypay_user')
}

// --------------- shared types ---------------

export interface Merchant {
  id: number
  mch_no: string
  name: string
  email: string
  role: string
  app_id: string
  app_secret?: string
  notify_url: string
  status: number
  remark: string
  created_at: string
  updated_at: string
}

export interface PlatformChannel {
  id: number
  channel: 'wechat' | 'alipay'
  status: number
  configured: boolean
  created_at: string
  updated_at: string
}

export interface Order {
  id: number
  order_no: string
  merchant_id: number
  merchant_order_no: string
  channel: string
  channel_order_no: string
  trade_type: string
  subject: string
  amount: number
  currency: string
  status: string
  created_at: string
  paid_at: string | null
}

export interface NotifyLog {
  id: number
  merchant_id: number
  order_no: string
  event_type: string
  notify_url: string
  http_status: number
  status: string
  retry_count: number
  next_retry_at: string | null
  last_error: string
  created_at: string
}

export interface MerchantChannel {
  id: number
  merchant_id: number
  channel: string
  status: number
}

export interface Settlement {
  id: number
  settlement_no: string
  merchant_id: number
  amount: number
  fee: number
  net_amount: number
  period_start: string
  period_end: string
  status: string
  remark: string
  paid_at: string | null
  created_at: string
}

export interface MerchantBalanceRow {
  merchant_id: number
  name: string
  mch_no: string
  total_income: number
  total_refund: number
  total_settled: number
  available: number
  period_start: string
}

export interface MerchantBalance {
  merchant_id: number
  total_income: number
  total_refund: number
  total_settled: number
  available: number
}

export interface DailyPayment {
  date: string
  amount: number
  count: number
}

export interface AdminDashboard {
  today: { order_count: number; paid_amount: number; refund_amount: number }
  overall: { total_merchants: number; total_orders: number; total_revenue: number }
  pending_settlement: number
  trend: DailyPayment[]
  recent_orders: Order[]
}

export interface MerchantDashboardData {
  today: { order_count: number; paid_amount: number; refund_amount: number }
  overall: { total_revenue: number; total_refund: number; available: number }
  trend: DailyPayment[]
  recent_orders: Order[]
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  size: number
}

// unwrap { code, data } envelope
function unwrap<T>(res: { data: { data: T } }): T {
  return res.data.data
}

// --------------- health / version ---------------

export const fetchVersion = () =>
  http.get('/health').then((res) => res.data.version as string)

// --------------- auth ---------------

export const authApi = {
  login: (email: string, password: string) =>
    http.post('/api/auth/login', { email, password }).then(unwrap<{
      token: string
      expires_in: number
      user: { id: number; name: string; email: string; role: string }
    }>),

  logout: () => http.post('/api/auth/logout'),

  me: () => http.get('/api/auth/me').then(unwrap<{
    id: number; name: string; email: string; role: string
  }>),
}

// --------------- admin ---------------

export const adminApi = {
  dashboard: (params?: { merchant_id?: number }) =>
    http.get('/api/admin/dashboard', { params }).then(unwrap<AdminDashboard>),

  // merchants
  listMerchants: (params: Record<string, any>) =>
    http.get('/api/admin/merchants', { params }).then(unwrap<PageResult<Merchant>>),

  createMerchant: (data: { name: string; email: string; notify_url?: string; remark?: string }) =>
    http.post('/api/admin/merchants', data).then(unwrap),

  updateMerchant: (id: number, data: Record<string, any>) =>
    http.put(`/api/admin/merchants/${id}`, data).then(unwrap),

  deleteMerchant: (id: number) =>
    http.delete(`/api/admin/merchants/${id}`),

  resetMerchantPassword: (id: number) =>
    http.post(`/api/admin/merchants/${id}/reset-password`).then(unwrap<{ email: string; password: string }>),

  // merchant channels
  listMerchantChannels: (merchantId: number) =>
    http.get(`/api/admin/merchants/${merchantId}/channels`).then(unwrap<MerchantChannel[]>),

  upsertMerchantChannel: (merchantId: number, channel: string, data: { status: number }) =>
    http.put(`/api/admin/merchants/${merchantId}/channels/${channel}`, data),

  // platform channels
  listPlatformChannels: () =>
    http.get('/api/admin/platform/channels').then(unwrap<PlatformChannel[]>),

  getPlatformChannel: (channel: string) =>
    http.get(`/api/admin/platform/channels/${channel}`).then(unwrap),

  upsertPlatformChannel: (channel: string, data: { config: Record<string, any>; status?: number }) =>
    http.put(`/api/admin/platform/channels/${channel}`, data),

  // orders
  listOrders: (params: Record<string, any>) =>
    http.get('/api/admin/orders', { params }).then(unwrap<PageResult<Order>>),

  createTestOrder: (data: Record<string, any>) =>
    http.post('/api/admin/orders/test', data).then(unwrap),

  refundOrder: (orderNo: string, data: { amount: number; reason?: string }) =>
    http.post(`/api/admin/orders/${orderNo}/refund`, data).then(unwrap),

  // notify logs
  listNotifyLogs: (params: Record<string, any>) =>
    http.get('/api/admin/notify_logs', { params }).then(unwrap<PageResult<NotifyLog>>),

  retryNotify: (id: number) =>
    http.post(`/api/admin/notify_logs/${id}/retry`),

  // utilities
  parseWechatCert: (pem: string) =>
    http.post('/api/admin/wechat/parse-cert', { pem }).then(unwrap<{
      serial_no: string; not_before: string; not_after: string; subject: string
    }>),

  // settlements
  merchantBalance: (merchantId: number) =>
    http.get(`/api/admin/merchants/${merchantId}/balance`).then(unwrap<MerchantBalance>),

  merchantPeriodBalance: (merchantId: number, start: string, end: string) =>
    http.get(`/api/admin/merchants/${merchantId}/period-balance`, { params: { start, end } }).then(unwrap<{ income: number; refund: number; amount: number }>),

  listSettlements: (params: Record<string, any>) =>
    http.get('/api/admin/settlements', { params }).then(unwrap<PageResult<Settlement>>),

  listMerchantBalances: () =>
    http.get('/api/admin/settlements/balances').then(unwrap<MerchantBalanceRow[]>),

  createSettlement: (data: { merchant_id: number; fee_rate: number; period_start: string; period_end: string; remark?: string }) =>
    http.post('/api/admin/settlements', data).then(unwrap),

  markSettlementPaid: (id: number) =>
    http.post(`/api/admin/settlements/${id}/pay`).then(unwrap),

  cancelSettlement: (id: number) =>
    http.post(`/api/admin/settlements/${id}/cancel`),

  // settings
  getSettings: () =>
    http.get('/api/admin/settings').then(unwrap<Record<string, string>>),

  updateSettings: (data: { platform_base: string }) =>
    http.put('/api/admin/settings', data),
}

// --------------- merchant self-service ---------------

export const merchantApi = {
  dashboard: () =>
    http.get('/api/merchant/dashboard').then(unwrap<MerchantDashboardData>),

  me: () =>
    http.get('/api/merchant/me').then(unwrap<Merchant>),

  updateProfile: (data: { name?: string; notify_url?: string }) =>
    http.put('/api/merchant/me', data),

  changePassword: (data: { old_password: string; new_password: string }) =>
    http.put('/api/merchant/me/password', data),

  listOrders: (params: Record<string, any>) =>
    http.get('/api/merchant/orders', { params }).then(unwrap<PageResult<Order>>),

  getOrder: (orderNo: string) =>
    http.get(`/api/merchant/orders/${orderNo}`).then(unwrap<Order>),

  listNotifyLogs: (params: Record<string, any>) =>
    http.get('/api/merchant/notify_logs', { params }).then(unwrap<PageResult<NotifyLog>>),

  balance: () =>
    http.get('/api/merchant/balance').then(unwrap<MerchantBalance>),

  listSettlements: (params: Record<string, any>) =>
    http.get('/api/merchant/settlements', { params }).then(unwrap<PageResult<Settlement>>),
}
