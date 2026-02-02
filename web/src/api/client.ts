import axios from 'axios'
import { getErrorMessage } from '@/utils/error'

// Detect if running in Tauri
const isTauri = typeof window !== 'undefined' && '__TAURI__' in window

// Use absolute URL in Tauri, relative URL in browser
const baseURL = isTauri ? 'http://localhost:23456/api/v1' : '/api/v1'

const api = axios.create({
  baseURL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor
api.interceptors.request.use(
  (config) => {
    // Add auth token if available
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }

    // Enhance error with translated message
    if (error.response) {
      error.translatedMessage = getErrorMessage(error)
    }

    return Promise.reject(error)
  }
)

export default api
