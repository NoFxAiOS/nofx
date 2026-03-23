import { httpClient } from './httpClient'

export interface EncryptedPayload {
  wrappedKey: string
  iv: string
  ciphertext: string
  aad?: string
  kid?: string
  ts?: number
}

export interface CryptoConfig {
  transport_encryption: boolean
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
  private static _transportEncryption: boolean | null = null

  static get transportEncryption(): boolean {
    return this._transportEncryption === true
  }

  static async initialize(publicKeyPEM: string) {
    if (this.publicKey && this.publicKeyPEM === publicKeyPEM) {
      return
    }
    this.publicKeyPEM = publicKeyPEM
    this.publicKey = await this.importPublicKey(publicKeyPEM)
  }

  static async fetchCryptoConfig(): Promise<CryptoConfig> {
    const result = await httpClient.get<CryptoConfig>('/api/crypto/config')
    if (!result.success || !result.data) {
      throw new Error(result.message || 'Failed to fetch crypto config')
    }
    this._transportEncryption = result.data.transport_encryption
    return result.data
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
      .replace(/\s+/g, '')

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

    const aesKey = await crypto.subtle.generateKey(
      {
        name: 'AES-GCM',
        length: 256,
      },
      true,
      ['encrypt']
    )

    const iv = crypto.getRandomValues(new Uint8Array(12))

    const ts = Math.floor(Date.now() / 1000)
    const aadObject = {
      userId: userId || '',
      sessionId: sessionId || '',
      ts,
      purpose: 'sensitive_data_encryption',
    }
    const aadString = JSON.stringify(aadObject)
    const aadBytes = new TextEncoder().encode(aadString)

    const plaintextBytes = new TextEncoder().encode(plaintext)
    const ciphertext = await crypto.subtle.encrypt(
      {
        name: 'AES-GCM',
        iv,
        additionalData: aadBytes,
        tagLength: 128,
      },
      aesKey,
      plaintextBytes
    )

    const rawAesKey = await crypto.subtle.exportKey('raw', aesKey)

    const wrappedKey = await crypto.subtle.encrypt(
      {
        name: 'RSA-OAEP',
      },
      this.publicKey,
      rawAesKey
    )

    return {
      wrappedKey: this.arrayBufferToBase64Url(wrappedKey),
      iv: this.arrayBufferToBase64Url(iv.buffer),
      ciphertext: this.arrayBufferToBase64Url(ciphertext),
      aad: this.arrayBufferToBase64Url(aadBytes.buffer),
      ts,
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
    const result = await httpClient.get<{
      public_key?: string
      transport_encryption?: boolean
    }>('/api/crypto/public-key')
    if (!result.success || !result.data) {
      throw new Error(result.message || 'Failed to fetch public key')
    }
    if (typeof result.data.transport_encryption === 'boolean') {
      this._transportEncryption = result.data.transport_encryption
    }
    return result.data.public_key || ''
  }

  static async decryptSensitiveData(
    payload: EncryptedPayload
  ): Promise<string> {
    const result = await httpClient.post<{ plaintext: string }>(
      '/api/crypto/decrypt',
      payload
    )

    if (!result.success || !result.data) {
      throw new Error(result.message || 'Decryption failed')
    }

    return result.data.plaintext
  }
}

export function generateObfuscation(): string {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join(
    ''
  )
}

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
