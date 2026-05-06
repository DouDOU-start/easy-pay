import axios from 'axios'

export const TOKEN_KEY = 'easypay_token'

export const api = axios.create({ baseURL: '/' })

api.interceptors.request.use((cfg) => {
  const t = localStorage.getItem(TOKEN_KEY)
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY)
      if (location.pathname !== '/login') location.href = '/login'
    }
    return Promise.reject(err)
  },
)

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
