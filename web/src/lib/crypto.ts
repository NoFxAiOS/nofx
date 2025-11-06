/**
 * 端到端加密模組
 * 使用混合加密: RSA-OAEP (密鑰交換) + AES-256-GCM (數據加密)
 */

// ==================== 核心加密函數 ====================

/**
 * 生成隨機混淆字串 (用於剪貼簿混淆)
 */
export function generateObfuscation(): string {
  const array = new Uint8Array(32);
  crypto.getRandomValues(array);
  return Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
}

/**
 * 使用伺服器公鑰加密私鑰
 * @param plaintext 明文私鑰
 * @param serverPublicKeyPEM 伺服器 RSA 公鑰 (PEM 格式)
 * @returns Base64 編碼的加密數據
 */
export async function encryptWithServerPublicKey(
  plaintext: string,
  serverPublicKeyPEM: string
): Promise<string> {
  try {
    // 1. 導入伺服器公鑰
    const publicKey = await importRSAPublicKey(serverPublicKeyPEM);

    // 2. 生成隨機 AES 密鑰 (256-bit)
    const aesKey = await crypto.subtle.generateKey(
      { name: 'AES-GCM', length: 256 },
      true,
      ['encrypt']
    );

    // 3. 使用 AES-GCM 加密數據
    const iv = crypto.getRandomValues(new Uint8Array(12)); // 96-bit nonce
    const encodedText = new TextEncoder().encode(plaintext);
    const encryptedData = await crypto.subtle.encrypt(
      { name: 'AES-GCM', iv },
      aesKey,
      encodedText
    );

    // 4. 導出 AES 密鑰並用 RSA 加密
    const exportedAESKey = await crypto.subtle.exportKey('raw', aesKey);
    const encryptedAESKey = await crypto.subtle.encrypt(
      { name: 'RSA-OAEP' },
      publicKey,
      exportedAESKey
    );

    // 5. 組合: [加密的 AES 密鑰長度(4字節)] + [加密的 AES 密鑰] + [IV] + [加密數據]
    const result = new Uint8Array(
      4 + encryptedAESKey.byteLength + iv.length + encryptedData.byteLength
    );
    const view = new DataView(result.buffer);
    view.setUint32(0, encryptedAESKey.byteLength, false); // 大端序
    result.set(new Uint8Array(encryptedAESKey), 4);
    result.set(iv, 4 + encryptedAESKey.byteLength);
    result.set(new Uint8Array(encryptedData), 4 + encryptedAESKey.byteLength + iv.length);

    // 6. Base64 編碼
    return arrayBufferToBase64(result);
  } catch (error) {
    console.error('加密失敗:', error);
    throw new Error('加密過程中發生錯誤，請檢查伺服器公鑰是否有效');
  }
}

/**
 * 導入 PEM 格式的 RSA 公鑰
 */
async function importRSAPublicKey(pem: string): Promise<CryptoKey> {
  // 移除 PEM header/footer 和換行符
  const pemContents = pem
    .replace(/-----BEGIN PUBLIC KEY-----/, '')
    .replace(/-----END PUBLIC KEY-----/, '')
    .replace(/\s/g, '');

  // Base64 解碼
  const binaryDer = base64ToArrayBuffer(pemContents);

  // 導入為 CryptoKey
  return crypto.subtle.importKey(
    'spki',
    binaryDer,
    {
      name: 'RSA-OAEP',
      hash: 'SHA-256',
    },
    true,
    ['encrypt']
  );
}

// ==================== 二階段輸入 UI ====================

export interface TwoStageInputResult {
  encryptedKey: string;
  obfuscationLog: string[]; // 混淆記錄（用於審計）
}

/**
 * 二階段私鑰輸入流程
 * @param serverPublicKey 伺服器公鑰
 * @returns 加密後的私鑰 + 混淆記錄
 */
export async function twoStagePrivateKeyInput(
  serverPublicKey: string
): Promise<TwoStageInputResult> {
  const obfuscationLog: string[] = [];

  return new Promise((resolve, reject) => {
    // 創建自定義 Modal
    const modal = createTwoStageModal(async (part1: string, part2: string) => {
      try {
        const fullKey = part1 + part2;

        // 驗證私鑰格式
        if (!validatePrivateKeyFormat(fullKey)) {
          throw new Error('私鑰格式不正確（應為 64 位十六進制或 0x 開頭）');
        }

        // 加密
        const encrypted = await encryptWithServerPublicKey(fullKey, serverPublicKey);

        // 清除敏感數據
        part1 = '';
        part2 = '';

        resolve({ encryptedKey: encrypted, obfuscationLog });
      } catch (error) {
        reject(error);
      }
    }, obfuscationLog);

    document.body.appendChild(modal);
  });
}

/**
 * 創建二階段輸入 Modal
 */
function createTwoStageModal(
  onSubmit: (part1: string, part2: string) => void,
  obfuscationLog: string[]
): HTMLElement {
  const modal = document.createElement('div');
  modal.style.cssText = `
    position: fixed; top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0,0,0,0.8); z-index: 10000;
    display: flex; align-items: center; justify-content: center;
  `;

  const content = document.createElement('div');
  content.style.cssText = `
    background: #1a1a2e; padding: 2rem; border-radius: 8px;
    max-width: 500px; width: 90%; color: white;
  `;

  let stage = 1;
  let part1 = '';

  const render = () => {
    if (stage === 1) {
      content.innerHTML = `
        <h2 style="margin-bottom: 1rem;">🔐 安全輸入 - 第一階段</h2>
        <p style="margin-bottom: 1rem; color: #888;">請輸入私鑰的<strong>前 32 位</strong>字符</p>
        <input
          id="stage1-input"
          type="password"
          placeholder="0x1234..."
          style="width: 100%; padding: 0.75rem; border-radius: 4px;
                 background: #0f0f1e; border: 1px solid #333; color: white;
                 font-family: monospace; font-size: 14px;"
          maxlength="34"
        />
        <button
          id="stage1-next"
          style="margin-top: 1rem; width: 100%; padding: 0.75rem;
                 background: #4CAF50; border: none; border-radius: 4px;
                 color: white; font-weight: bold; cursor: pointer;"
        >下一步 →</button>
        <button
          id="cancel"
          style="margin-top: 0.5rem; width: 100%; padding: 0.5rem;
                 background: transparent; border: 1px solid #555; border-radius: 4px;
                 color: #888; cursor: pointer;"
        >取消</button>
      `;

      const input = content.querySelector('#stage1-input') as HTMLInputElement;
      const nextBtn = content.querySelector('#stage1-next') as HTMLButtonElement;
      const cancelBtn = content.querySelector('#cancel') as HTMLButtonElement;

      input.focus();
      input.addEventListener('input', () => {
        nextBtn.disabled = input.value.length < 10;
      });

      nextBtn.addEventListener('click', async () => {
        part1 = input.value;
        input.value = ''; // 立即清除

        // 生成混淆字串並強制複製
        const obfuscation = generateObfuscation();
        await navigator.clipboard.writeText(obfuscation);
        obfuscationLog.push(`Stage1: ${new Date().toISOString()}`);

        alert('⚠️ 已複製混淆字串到剪貼簿\n\n請在任意地方貼上一次（避免監控），然後點擊確定繼續');
        stage = 2;
        render();
      });

      cancelBtn.addEventListener('click', () => {
        modal.remove();
      });
    } else if (stage === 2) {
      content.innerHTML = `
        <h2 style="margin-bottom: 1rem;">🔐 安全輸入 - 第二階段</h2>
        <p style="margin-bottom: 1rem; color: #888;">請輸入私鑰的<strong>剩餘字符</strong></p>
        <input
          id="stage2-input"
          type="password"
          placeholder="...5678"
          style="width: 100%; padding: 0.75rem; border-radius: 4px;
                 background: #0f0f1e; border: 1px solid #333; color: white;
                 font-family: monospace; font-size: 14px;"
          maxlength="34"
        />
        <button
          id="stage2-submit"
          style="margin-top: 1rem; width: 100%; padding: 0.75rem;
                 background: #2196F3; border: none; border-radius: 4px;
                 color: white; font-weight: bold; cursor: pointer;"
        >🔒 加密並提交</button>
        <button
          id="back"
          style="margin-top: 0.5rem; width: 100%; padding: 0.5rem;
                 background: transparent; border: 1px solid #555; border-radius: 4px;
                 color: #888; cursor: pointer;"
        >← 返回上一步</button>
      `;

      const input = content.querySelector('#stage2-input') as HTMLInputElement;
      const submitBtn = content.querySelector('#stage2-submit') as HTMLButtonElement;
      const backBtn = content.querySelector('#back') as HTMLButtonElement;

      input.focus();
      submitBtn.addEventListener('click', async () => {
        const part2 = input.value;
        input.value = ''; // 立即清除

        obfuscationLog.push(`Stage2: ${new Date().toISOString()}`);

        modal.remove();
        onSubmit(part1, part2);
      });

      backBtn.addEventListener('click', () => {
        stage = 1;
        render();
      });
    }
  };

  render();
  modal.appendChild(content);
  return modal;
}

/**
 * 驗證私鑰格式
 */
function validatePrivateKeyFormat(key: string): boolean {
  // EVM 私鑰: 64 位十六進制 (可選 0x 前綴)
  const evmPattern = /^(0x)?[0-9a-fA-F]{64}$/;
  return evmPattern.test(key);
}

// ==================== 工具函數 ====================

function arrayBufferToBase64(buffer: ArrayBuffer | Uint8Array): string {
  const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

/**
 * 從伺服器獲取公鑰
 */
export async function fetchServerPublicKey(): Promise<string> {
  const response = await fetch('/api/crypto/public-key');
  if (!response.ok) {
    throw new Error('無法獲取伺服器公鑰');
  }
  const data = await response.json();
  return data.public_key;
}
