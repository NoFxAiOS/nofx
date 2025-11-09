/**
 * HTTP Client with unified error handling and 401 interception
 *
 * Features:
 * - Unified fetch wrapper
 * - Automatic 401 token expiration handling
 * - Auth state cleanup on unauthorized
 * - Automatic redirect to login page
 */

export class HttpClient {
  // Singleton flag to prevent duplicate 401 handling
  private static isHandling401 = false

  /**
   * Reset 401 handling flag (call after successful login)
   */
  public reset401Flag(): void {
    HttpClient.isHandling401 = false
  }

  /**
   * Show login required notification to user
   */
  private showLoginRequiredNotification(): void {
    // Create notification element
    const notification = document.createElement('div')
    notification.style.cssText = `
      position: fixed;
      top: 20px;
      left: 50%;
      transform: translateX(-50%);
      background: linear-gradient(135deg, #F0B90B 0%, #FCD535 100%);
      color: #0B0E11;
      padding: 16px 24px;
      border-radius: 8px;
      font-size: 16px;
      font-weight: bold;
      z-index: 10000;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      animation: slideDown 0.3s ease-out;
    `
    notification.textContent = '⚠️ 登录已过期，请先登录'

    // Add slide down animation
    const style = document.createElement('style')
    style.textContent = `
      @keyframes slideDown {
        from {
          opacity: 0;
          transform: translateX(-50%) translateY(-20px);
        }
        to {
          opacity: 1;
          transform: translateX(-50%) translateY(0);
        }
      }
    `
    document.head.appendChild(style)

    // Add to page
    document.body.appendChild(notification)

    // Auto remove after animation
    setTimeout(() => {
      notification.style.animation = 'slideDown 0.3s ease-out reverse'
      setTimeout(() => {
        document.body.removeChild(notification)
        document.head.removeChild(style)
      }, 300)
    }, 1800)
  }

  /**
   * Response interceptor - handles common HTTP errors
   *
   * @param response - Fetch Response object
   * @returns Response if successful
   * @throws Error with user-friendly message
   */
  private async handleResponse(response: Response): Promise<Response> {
    // Handle 401 Unauthorized - Token expired or invalid
    if (response.status === 401) {
      // Prevent duplicate 401 handling when multiple API calls fail simultaneously
      if (HttpClient.isHandling401) {
        throw new Error('登录已过期，请重新登录')
      }

      // Set flag to prevent race conditions
      HttpClient.isHandling401 = true

      // Clean up local storage
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_user')

      // Notify global listeners (AuthContext will react to this)
      window.dispatchEvent(new Event('unauthorized'))

      // Show user-friendly notification (only once)
      this.showLoginRequiredNotification()

      // Delay redirect to let user see the notification
      setTimeout(() => {
        // Only redirect if not already on login page
        if (!window.location.pathname.includes('/login')) {
          // Save current location for post-login redirect
          const returnUrl = window.location.pathname + window.location.search
          if (returnUrl !== '/login' && returnUrl !== '/') {
            sessionStorage.setItem('returnUrl', returnUrl)
          }

          window.location.href = '/login'
        }
        // Note: No need to reset flag since we're redirecting
      }, 1500) // 1.5秒延迟,让用户看到提示

      throw new Error('登录已过期，请重新登录')
    }

    // Handle other common errors
    if (response.status === 403) {
      throw new Error('没有权限访问此资源')
    }

    if (response.status === 404) {
      throw new Error('请求的资源不存在')
    }

    if (response.status >= 500) {
      throw new Error('服务器错误，请稍后重试')
    }

    return response
  }

  /**
   * GET request
   */
  async get(url: string, headers?: Record<string, string>): Promise<Response> {
    const response = await fetch(url, {
      method: 'GET',
      headers,
    })
    return this.handleResponse(response)
  }

  /**
   * POST request
   */
  async post(
    url: string,
    body?: any,
    headers?: Record<string, string>
  ): Promise<Response> {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...headers,
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    return this.handleResponse(response)
  }

  /**
   * PUT request
   */
  async put(
    url: string,
    body?: any,
    headers?: Record<string, string>
  ): Promise<Response> {
    const response = await fetch(url, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...headers,
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    return this.handleResponse(response)
  }

  /**
   * DELETE request
   */
  async delete(
    url: string,
    headers?: Record<string, string>
  ): Promise<Response> {
    const response = await fetch(url, {
      method: 'DELETE',
      headers,
    })
    return this.handleResponse(response)
  }

  /**
   * Generic request method for custom configurations
   */
  async request(url: string, options: RequestInit = {}): Promise<Response> {
    const response = await fetch(url, options)
    return this.handleResponse(response)
  }
}

// Export singleton instance
export const httpClient = new HttpClient()

// Export helper function to reset 401 flag
export const reset401Flag = () => httpClient.reset401Flag()
