export interface EncryptedPayload {
  wrappedKey: string // RSA-OAEP(K)
  iv: string // 12 bytes
  ciphertext: string // AES-GCM 输出(含 tag)
  aad?: string // 可选：额外认证数据
  kid?: string // 可选：服务端公钥标识
  ts?: number // 可选：unix 秒，用于重放保护
}

export interface WebCryptoEnvironmentInfo {
  isBrowser: boolean
  isSecureContext: boolean
  hasSubtleCrypto: boolean
  origin?: string
  protocol?: string
  hostname?: string
  isLocalhost?: boolean
}

export class CryptoService {
  private static publicKey: CryptoKey | null = null
  private static publicKeyPEM: string | null = null

  static async initialize(publicKeyPEM: string) {
    if (this.publicKey && this.publicKeyPEM === publicKeyPEM) {
      return
    }
    this.publicKeyPEM = publicKeyPEM
    this.publicKey = await this.importPublicKey(publicKeyPEM)
  }

  private static async importPublicKey(pem: string): Promise<CryptoKey> {
    const pemHeader = '-----BEGIN PUBLIC KEY-----'
    const pemFooter = '-----END PUBLIC KEY-----'
    const headerIndex = pem.indexOf(pemHeader)
    const footerIndex = pem.indexOf(pemFooter)

    if (
      headerIndex === -1 ||
      footerIndex === -1 ||
      headerIndex >= footerIndex
    ) {
      throw new Error('Invalid PEM formatted public key')
    }

    const pemContents = pem
      .substring(headerIndex + pemHeader.length, footerIndex)
      .replace(/\s+/g, '') // 移除所有空白字符（包括换行符、空格等）

    const binaryDerString = atob(pemContents)
    const binaryDer = new Uint8Array(binaryDerString.length)
    for (let i = 0; i < binaryDerString.length; i++) {
      binaryDer[i] = binaryDerString.charCodeAt(i)
    }

    return crypto.subtle.importKey(
      'spki',
      binaryDer,
      {
        name: 'RSA-OAEP',
        hash: 'SHA-256',
      },
      false,
      ['encrypt']
    )
  }

  static async encryptSensitiveData(
    plaintext: string,
    userId?: string,
    sessionId?: string
  ): Promise<EncryptedPayload> {
    if (!this.publicKey) {
      throw new Error(
        'Crypto service not initialized. Call initialize() first.'
      )
    }

    // 1. 生成 256-bit AES 密钥
    const aesKey = await crypto.subtle.generateKey(
      {
        name: 'AES-GCM',
        length: 256,
      },
      true,
      ['encrypt']
    )

    // 2. 生成 12 字节随机 IV
    const iv = crypto.getRandomValues(new Uint8Array(12))

    // 3. 准备 AAD (额外认证数据)
    const ts = Math.floor(Date.now() / 1000)
    const aadObject = {
      userId: userId || '',
      sessionId: sessionId || '',
      ts: ts,
      purpose: 'sensitive_data_encryption',
    }
    const aadString = JSON.stringify(aadObject)
    const aadBytes = new TextEncoder().encode(aadString)

    // 4. 使用 AES-GCM 加密数据
    const plaintextBytes = new TextEncoder().encode(plaintext)
    const ciphertext = await crypto.subtle.encrypt(
      {
        name: 'AES-GCM',
        iv: iv,
        additionalData: aadBytes,
        tagLength: 128, // 16 bytes tag
      },
      aesKey,
      plaintextBytes
    )

    // 5. 导出 AES 密钥
    const rawAesKey = await crypto.subtle.exportKey('raw', aesKey)

    // 6. 使用 RSA-OAEP 加密 AES 密钥
    const wrappedKey = await crypto.subtle.encrypt(
      {
        name: 'RSA-OAEP',
      },
      this.publicKey,
      rawAesKey
    )

    // 7. 编码为 base64url
    return {
      wrappedKey: this.arrayBufferToBase64Url(wrappedKey),
      iv: this.arrayBufferToBase64Url(iv.buffer),
      ciphertext: this.arrayBufferToBase64Url(ciphertext),
      aad: this.arrayBufferToBase64Url(aadBytes.buffer),
      ts: ts,
    }
  }

  private static arrayBufferToBase64Url(buffer: ArrayBuffer): string {
    const bytes = new Uint8Array(buffer)
    let binary = ''
    for (let i = 0; i < bytes.length; i++) {
      binary += String.fromCharCode(bytes[i])
    }
    return btoa(binary)
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=/g, '')
  }

  static async fetchPublicKey(): Promise<string> {
    const response = await fetch('/api/crypto/public-key')
    if (!response.ok) {
      throw new Error(`Failed to fetch public key: ${response.statusText}`)
    }
    const data = await response.json()
    return data.public_key
  }

  static async decryptSensitiveData(
    payload: EncryptedPayload
  ): Promise<string> {
    const response = await fetch('/api/crypto/decrypt', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    })

    if (!response.ok) {
      throw new Error(`Decryption failed: ${response.statusText}`)
    }

    const result = await response.json()
    return result.plaintext
  }
}

// 生成混淆字符串（用于剪贴板混淆）
export function generateObfuscation(): string {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join(
    ''
  )
}

// 验证私钥格式
export function validatePrivateKeyFormat(
  value: string,
  expectedLength: number = 64
): boolean {
  const normalized = value.startsWith('0x') ? value.slice(2) : value
  if (normalized.length !== expectedLength) {
    return false
  }
  return /^[0-9a-fA-F]+$/.test(normalized)
}

export function diagnoseWebCryptoEnvironment(): WebCryptoEnvironmentInfo {
  if (typeof window === 'undefined') {
    return {
      isBrowser: false,
      isSecureContext: false,
      hasSubtleCrypto: false,
    }
  }

  const { location } = window
  const hostname = location?.hostname
  const protocol = location?.protocol
  const origin = location?.origin
  const isLocalhost = hostname
    ? ['localhost', '127.0.0.1', '::1'].includes(hostname)
    : false

  const secureContext =
    typeof window.isSecureContext === 'boolean'
      ? window.isSecureContext
      : protocol === 'https:' || (protocol === 'http:' && isLocalhost)

  const hasSubtleCrypto =
    typeof window.crypto !== 'undefined' &&
    typeof window.crypto.subtle !== 'undefined'

  return {
    isBrowser: true,
    isSecureContext: secureContext,
    hasSubtleCrypto,
    origin: origin || undefined,
    protocol: protocol || undefined,
    hostname,
    isLocalhost,
  }
}
