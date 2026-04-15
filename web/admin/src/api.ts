import axios from 'axios'

export const api = axios.create({ baseURL: '/' })

api.interceptors.request.use((cfg) => {
  const t = localStorage.getItem('easypay_token')
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('easypay_token')
      if (location.pathname !== '/login') location.href = '/login'
    }
    return Promise.reject(err)
  },
)
