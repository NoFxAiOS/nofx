/**
 * HTTP Client with Axios
 *
 * Features:
 * - Axios-based unified request wrapper
 * - Automatic error interception and toast notifications
 * - Network errors and system errors are intercepted and shown via toast
 * - Only business logic errors are returned to the caller
 * - Automatic 401 token expiration handling
 */

import axios, { AxiosInstance, AxiosError, AxiosResponse } from 'axios'
import { toast } from 'sonner'

/**
 * Business response format - only business errors reach the caller
 */
export interface ApiResponse<T = any> {
  success: boolean
  data?: T
  message?: string
}

/**
 * HTTP Client Class
 */
export class HttpClient {
  private axiosInstance: AxiosInstance
  private static isHandling401 = false

  constructor() {
    // Create axios instance
    this.axiosInstance = axios.create({
      baseURL: '/',
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    })

    // Setup interceptors
    this.setupInterceptors()
  }

  /**
   * Reset 401 handling flag (call after successful login)
   */
  public reset401Flag(): void {
    HttpClient.isHandling401 = false
  }

  /**
   * Setup request and response interceptors
   */
  private setupInterceptors(): void {
    // Request interceptor - add auth token
    this.axiosInstance.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('auth_token')
        if (token) {
          config.headers.Authorization = `Bearer ${token}`
        }
        return config
      },
      (error) => {
        return Promise.reject(error)
      }
    )

    // Response interceptor - handle errors
    this.axiosInstance.interceptors.response.use(
      (response: AxiosResponse) => {
        // Success response - pass through
        return response
      },
      (error: AxiosError) => {
        return this.handleError(error)
      }
    )
  }

  /**
   * Handle different types of errors
   * Network and system errors are intercepted and shown via toast
   * Only business errors are returned to caller
   */
  private async handleError(error: AxiosError): Promise<any> {
    // Network error (no response from server)
    if (!error.response) {
      toast.error('Network error - Please check your connection', {
        description: 'Unable to reach the server',
      })
      throw new Error('Network error')
    }

    const { status } = error.response as AxiosResponse<{
      error?: string
      message?: string
    }>

    // Handle 401 Unauthorized
    if (status === 401) {
      if (HttpClient.isHandling401) {
        throw new Error('Session expired')
      }

      HttpClient.isHandling401 = true

      // Clean up
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_user')

      // Notify global listeners
      window.dispatchEvent(new Event('unauthorized'))

      // Only redirect if not already on login page
      if (!window.location.pathname.includes('/login')) {
        const returnUrl = window.location.pathname + window.location.search
        if (returnUrl !== '/login' && returnUrl !== '/') {
          sessionStorage.setItem('returnUrl', returnUrl)
        }

        sessionStorage.setItem('from401', 'true')
        window.location.href = '/login'

        // Return pending promise
        return new Promise(() => {})
      }

      throw new Error('Session expired')
    }

    // Handle 403 Forbidden - system error
    if (status === 403) {
      toast.error('Permission Denied', {
        description: 'You do not have permission to access this resource',
      })
      throw new Error('Permission denied')
    }

    // Handle 404 Not Found - system error
    if (status === 404) {
      toast.error('API Not Found', {
        description: 'The requested endpoint does not exist (404)',
      })
      throw new Error('API not found')
    }

    // Handle 500+ Server Error - system error
    if (status >= 500) {
      toast.error('Server Error', {
        description: 'Please try again later or contact support',
      })
      throw new Error('Server error')
    }

    // 4xx errors (except 401/403/404) are business logic errors
    // Return them to the caller for handling
    return Promise.reject(error)
  }

  /**
   * Generic JSON request with standardized response
   * System/network errors are already intercepted and shown via toast
   * Only business errors are returned
   */
  async request<T = any>(
    url: string,
    options: {
      method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
      data?: any
      params?: any
      headers?: Record<string, string>
    } = {}
  ): Promise<ApiResponse<T>> {
    try {
      const response = await this.axiosInstance.request<T>({
        url,
        method: options.method || 'GET',
        data: options.data,
        params: options.params,
        headers: options.headers,
      })

      // Success
      return {
        success: true,
        data: response.data,
        message: (response.data as any)?.message,
      }
    } catch (error) {
      // If we get here, it's a business logic error (4xx except 401/403/404)
      // System errors were already intercepted and toasted
      if (axios.isAxiosError(error) && error.response) {
        const errorData = error.response.data as any
        return {
          success: false,
          message: errorData?.error || errorData?.message || 'Operation failed',
        }
      }

      // Network error or other exception (already toasted)
      throw error
    }
  }

  /**
   * GET request
   */
  async get<T = any>(
    url: string,
    params?: any,
    headers?: Record<string, string>
  ): Promise<ApiResponse<T>> {
    return this.request<T>(url, { method: 'GET', params, headers })
  }

  /**
   * POST request
   */
  async post<T = any>(
    url: string,
    data?: any,
    headers?: Record<string, string>
  ): Promise<ApiResponse<T>> {
    return this.request<T>(url, { method: 'POST', data, headers })
  }

  /**
   * PUT request
   */
  async put<T = any>(
    url: string,
    data?: any,
    headers?: Record<string, string>
  ): Promise<ApiResponse<T>> {
    return this.request<T>(url, { method: 'PUT', data, headers })
  }

  /**
   * DELETE request
   */
  async delete<T = any>(
    url: string,
    headers?: Record<string, string>
  ): Promise<ApiResponse<T>> {
    return this.request<T>(url, { method: 'DELETE', headers })
  }

  /**
   * PATCH request
   */
  async patch<T = any>(
    url: string,
    data?: any,
    headers?: Record<string, string>
  ): Promise<ApiResponse<T>> {
    return this.request<T>(url, { method: 'PATCH', data, headers })
  }
}

// Export singleton instance
export const httpClient = new HttpClient()

// Export helper function to reset 401 flag
export const reset401Flag = () => httpClient.reset401Flag()
