export interface EncryptedPayload {
  wrappedKey: string;     // RSA-OAEP(K)
  iv: string;             // 12 bytes
  ciphertext: string;     // AES-GCM 输出(含 tag)
  aad?: string;           // 可选：额外认证数据
  kid?: string;           // 可选：服务端公钥标识
  ts?: number;            // 可选：unix 秒，用于重放保护
}

export class CryptoService {
  private static publicKey: CryptoKey | null = null;
  private static publicKeyPEM: string | null = null;

  static async initialize(publicKeyPEM: string) {
    // 检查 Web Crypto API 是否可用
    if (!window.crypto || !window.crypto.subtle) {
      throw new Error('Web Crypto API is not available. Please use HTTPS or localhost to access the application.');
    }
    
    if (this.publicKey && this.publicKeyPEM === publicKeyPEM) {
      return;
    }
    this.publicKeyPEM = publicKeyPEM;
    this.publicKey = await this.importPublicKey(publicKeyPEM);
  }

  private static async importPublicKey(pem: string): Promise<CryptoKey> {
    const pemHeader = '-----BEGIN PUBLIC KEY-----';
    const pemFooter = '-----END PUBLIC KEY-----';
    const headerIndex = pem.indexOf(pemHeader);
    const footerIndex = pem.indexOf(pemFooter);

    if (headerIndex === -1 || footerIndex === -1 || headerIndex >= footerIndex) {
      throw new Error('Invalid PEM formatted public key');
    }

    const pemContents = pem
      .substring(headerIndex + pemHeader.length, footerIndex)
      .replace(/\s+/g, ''); // 移除所有空白字符（包括换行符、空格等）
    
    const binaryDerString = atob(pemContents);
    const binaryDer = new Uint8Array(binaryDerString.length);
    for (let i = 0; i < binaryDerString.length; i++) {
      binaryDer[i] = binaryDerString.charCodeAt(i);
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
    );
  }

  static async encryptSensitiveData(
    plaintext: string,
    userId?: string,
    sessionId?: string
  ): Promise<EncryptedPayload> {
    if (!this.publicKey) {
      throw new Error('Crypto service not initialized. Call initialize() first.');
    }

    // 1. 生成 256-bit AES 密钥
    const aesKey = await crypto.subtle.generateKey(
      {
        name: 'AES-GCM',
        length: 256,
      },
      true,
      ['encrypt']
    );

    // 2. 生成 12 字节随机 IV
    const iv = crypto.getRandomValues(new Uint8Array(12));

    // 3. 准备 AAD (额外认证数据)
    const ts = Math.floor(Date.now() / 1000);
    const aadObject = {
      userId: userId || '',
      sessionId: sessionId || '',
      ts: ts,
      purpose: 'sensitive_data_encryption'
    };
    const aadString = JSON.stringify(aadObject);
    const aadBytes = new TextEncoder().encode(aadString);

    // 4. 使用 AES-GCM 加密数据
    const plaintextBytes = new TextEncoder().encode(plaintext);
    const ciphertext = await crypto.subtle.encrypt(
      {
        name: 'AES-GCM',
        iv: iv,
        additionalData: aadBytes,
        tagLength: 128, // 16 bytes tag
      },
      aesKey,
      plaintextBytes
    );

    // 5. 导出 AES 密钥
    const aesKeyRaw = await crypto.subtle.exportKey('raw', aesKey);

    // 6. 使用 RSA-OAEP 加密 AES 密钥
    const wrappedKey = await crypto.subtle.encrypt(
      {
        name: 'RSA-OAEP',
      },
      this.publicKey,
      aesKeyRaw
    );

    // 7. 转换为 base64url 格式
    return {
      wrappedKey: this.arrayBufferToBase64Url(wrappedKey),
      iv: this.arrayBufferToBase64Url(iv),
      ciphertext: this.arrayBufferToBase64Url(ciphertext),
      aad: this.arrayBufferToBase64Url(aadBytes),
      kid: 'rsa-key-2025-11-05',
      ts: ts,
    };
  }

  private static arrayBufferToBase64Url(buffer: ArrayBuffer | Uint8Array): string {
    const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
    let binary = '';
    for (let i = 0; i < bytes.byteLength; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary)
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=/g, '');
  }

  static async encryptWalletPrivateKey(privateKey: string, userId?: string, sessionId?: string): Promise<EncryptedPayload> {
    return this.encryptSensitiveData(privateKey, userId, sessionId);
  }

  static async encryptExchangeSecret(secretKey: string, userId?: string, sessionId?: string): Promise<EncryptedPayload> {
    return this.encryptSensitiveData(secretKey, userId, sessionId);
  }
}
