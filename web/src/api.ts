import axios from 'axios'

export const ADMIN_TOKEN_KEY = 'easypay_token'
export const MERCHANT_TOKEN_KEY = 'easypay_merchant_token'

function createAuthedApi(tokenKey: string, loginPath: string) {
  const client = axios.create({ baseURL: '/' })

  client.interceptors.request.use((cfg) => {
    const t = localStorage.getItem(tokenKey)
    if (t) cfg.headers.Authorization = `Bearer ${t}`
    return cfg
  })

  client.interceptors.response.use(
    (res) => res,
    (err) => {
      if (err.response?.status === 401) {
        localStorage.removeItem(tokenKey)
        if (location.pathname !== loginPath) location.href = loginPath
      }
      return Promise.reject(err)
    },
  )

  return client
}

export const api = createAuthedApi(ADMIN_TOKEN_KEY, '/login')
export const merchantApi = createAuthedApi(MERCHANT_TOKEN_KEY, '/merchant-login')
