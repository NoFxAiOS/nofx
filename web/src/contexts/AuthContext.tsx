import React, { createContext, useContext, useState, useEffect } from 'react'
import { flushSync } from 'react-dom'
import { getSystemConfig } from '../lib/config'
import { reset401Flag, httpClient } from '../lib/httpClient'
import { getPostAuthPath, setUserMode, type UserMode } from '../lib/onboarding'
import { useLanguage } from './LanguageContext'

interface User {
  id: string
  email: string
}

interface AuthContextType {
  user: User | null
  token: string | null
  login: (
    email: string,
    password: string,
    mode?: UserMode
  ) => Promise<{
    success: boolean
    message?: string
  }>
  loginAdmin: (password: string) => Promise<{
    success: boolean
    message?: string
  }>
  register: (
    email: string,
    password: string,
    betaCode?: string,
    mode?: UserMode
  ) => Promise<{ success: boolean; message?: string }>
  resetPassword: (
    email: string,
    newPassword: string
  ) => Promise<{ success: boolean; message?: string }>
  logout: () => void
  isLoading: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const { language } = useLanguage()
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // Reset 401 flag on page load to allow fresh 401 handling
    reset401Flag()

    // Check if admin mode is active (uses cached system config)
    getSystemConfig()
      .then(() => {
        // No longer simulate login in admin mode; check local storage uniformly
        const savedToken = localStorage.getItem('auth_token')
        const savedUser = localStorage.getItem('auth_user')
        if (savedToken && savedUser) {
          setToken(savedToken)
          setUser(JSON.parse(savedUser))
        }

        setIsLoading(false)
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
        // On error, continue checking local storage
        const savedToken = localStorage.getItem('auth_token')
        const savedUser = localStorage.getItem('auth_user')

        if (savedToken && savedUser) {
          setToken(savedToken)
          setUser(JSON.parse(savedUser))
        }
        setIsLoading(false)
      })
  }, [])

  // Listen for unauthorized events from httpClient (401 responses)
  useEffect(() => {
    const handleUnauthorized = () => {
      console.log('Unauthorized event received - clearing auth state')
      // Clear auth state when 401 is detected
      setUser(null)
      setToken(null)
      // Note: localStorage cleanup is already done in httpClient
    }

    window.addEventListener('unauthorized', handleUnauthorized)

    return () => {
      window.removeEventListener('unauthorized', handleUnauthorized)
    }
  }, [])

  const handlePostAuthSuccess = (
    authToken: string,
    userInfo: User,
    mode?: UserMode
  ) => {
    reset401Flag()

    if (mode) {
      setUserMode(mode)
    }

    localStorage.setItem('auth_token', authToken)
    localStorage.setItem('auth_user', JSON.stringify(userInfo))
    localStorage.setItem('user_id', userInfo.id)
    flushSync(() => {
      setToken(authToken)
      setUser(userInfo)
    })

    const returnUrl = sessionStorage.getItem('returnUrl')
    const nextPath = returnUrl || getPostAuthPath(mode)
    if (returnUrl) {
      sessionStorage.removeItem('returnUrl')
    }

    window.history.pushState({}, '', nextPath)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  const login = async (email: string, password: string, mode?: UserMode) => {
    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      })

      const data = await response.json()

      if (response.ok) {
        if (data.token) {
          const userInfo = { id: data.user_id, email: data.email }
          handlePostAuthSuccess(data.token, userInfo, mode)

          return { success: true, message: data.message }
        }

        // Unexpected success response
        return { success: false, message: data.message || 'Unexpected login response' }
      } else {
        return {
          success: false,
          message: data.error,
        }
      }
    } catch (error) {
      return { success: false, message: 'Login failed, please try again' }
    }
  }

  const loginAdmin = async (password: string) => {
    try {
      const response = await fetch('/api/admin-login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })
      const data = await response.json()
      if (response.ok) {
        // Reset 401 flag on successful login
        reset401Flag()

        const userInfo = {
          id: data.user_id || 'admin',
          email: data.email || 'admin@localhost',
        }
        localStorage.setItem('auth_token', data.token)
        localStorage.setItem('auth_user', JSON.stringify(userInfo))
        flushSync(() => {
          setToken(data.token)
          setUser(userInfo)
        })

        // Check and redirect to returnUrl if exists
        const returnUrl = sessionStorage.getItem('returnUrl')
        if (returnUrl) {
          sessionStorage.removeItem('returnUrl')
          window.history.pushState({}, '', returnUrl)
          window.dispatchEvent(new PopStateEvent('popstate'))
        } else {
          // Redirect to dashboard
          window.history.pushState({}, '', '/dashboard')
          window.dispatchEvent(new PopStateEvent('popstate'))
        }
        return { success: true }
      } else {
        return { success: false, message: data.error || 'Login failed' }
      }
    } catch (e) {
      return { success: false, message: 'Login failed, please try again' }
    }
  }

  const register = async (
    email: string,
    password: string,
    betaCode?: string,
    mode?: UserMode
  ) => {
    const requestBody: {
      email: string
      password: string
      beta_code?: string
      lang?: string
    } = { email, password, lang: language }
    if (betaCode) {
      requestBody.beta_code = betaCode
    }

    try {
      const result = await httpClient.post<{
        token: string
        user_id: string
        email: string
        message: string
      }>('/api/register', requestBody)

      if (result.success && result.data) {
        const userInfo = { id: result.data.user_id, email: result.data.email }
        handlePostAuthSuccess(result.data.token, userInfo, mode)

        return {
          success: true,
          message: result.message || result.data.message,
        }
      }

      // Only business errors reach here (system/network errors were intercepted)
      return {
        success: false,
        message: result.message || 'Registration failed',
      }
    } catch (error) {
      console.error('Auth register error:', error);
      // Re-throw if it's a critical error, or return structured error
      // Since httpClient throws on 500, we should return a structured error response
      // to let the UI display it gracefully without crashing.
      return {
        success: false,
        message: error instanceof Error ? error.message : 'Detailed server error'
      }
    }
  }

  const resetPassword = async (email: string, newPassword: string) => {
    try {
      const response = await fetch('/api/reset-password', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          email,
          new_password: newPassword,
        }),
      })

      const data = await response.json()

      if (response.ok) {
        return { success: true, message: data.message }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: 'Password reset failed, please try again' }
    }
  }

  const logout = () => {
    const savedToken = localStorage.getItem('auth_token')
    if (savedToken) {
      fetch('/api/logout', {
        method: 'POST',
        headers: { Authorization: `Bearer ${savedToken}` },
      }).catch(() => {
        /* ignore network errors on logout */
      })
    }
    setUser(null)
    setToken(null)
    localStorage.removeItem('auth_token')
    localStorage.removeItem('auth_user')
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        login,
        loginAdmin,
        register,
        resetPassword,
        logout,
        isLoading,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
