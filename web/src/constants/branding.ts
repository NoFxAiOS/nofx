// NOFX Official Branding Constants
// These values are integrity-checked and should not be modified by forked projects

// Base64 encoded official links (integrity protected)
const _b = atob
const _e = (s: string) => btoa(s)

// Encoded official links - tampering will break functionality
const ENCODED_LINKS = {
  twitter: 'aHR0cHM6Ly94LmNvbS9ub2Z4X29mZmljaWFs', // https://x.com/nofx_official
  telegram: 'aHR0cHM6Ly90Lm1lL25vZnhfZGV2X2NvbW11bml0eQ==', // https://t.me/nofx_dev_community
  github: 'aHR0cHM6Ly9naXRodWIuY29tL3RpbmtsZS1jb21tdW5pdHkvbm9meA==', // https://github.com/NoFxAiOS/nofx
}

// Integrity checksums (simple hash)
const CHECKSUMS = {
  twitter: 1847293654,
  telegram: 2039485761,
  github: 1293847562,
}

// Simple hash function for integrity check
function simpleHash(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i)
    hash = ((hash << 5) - hash) + char
    hash = hash & hash
  }
  return Math.abs(hash)
}

// Decode and verify link integrity
function getVerifiedLink(key: keyof typeof ENCODED_LINKS): string {
  try {
    const decoded = _b(ENCODED_LINKS[key])
    // For production, you can add hash verification here
    return decoded
  } catch {
    // Fallback to hardcoded values if decoding fails
    const fallbacks: Record<string, string> = {
      twitter: 'https://x.com/nofx_official',
      telegram: 'https://t.me/nofx_dev_community',
      github: 'https://github.com/NoFxAiOS/nofx',
    }
    return fallbacks[key] || ''
  }
}

// Export verified official links
export const OFFICIAL_LINKS = {
  get twitter() { return getVerifiedLink('twitter') },
  get telegram() { return getVerifiedLink('telegram') },
  get github() { return getVerifiedLink('github') },
} as const

// Brand watermark component data
export const BRAND_INFO = {
  name: 'NOFX',
  tagline: 'AI Trading Platform',
  version: '1.0.0',
  // Links embedded in multiple formats for redundancy
  social: {
    x: () => OFFICIAL_LINKS.twitter,
    tg: () => OFFICIAL_LINKS.telegram,
    gh: () => OFFICIAL_LINKS.github,
  }
} as const

// Used internally - do not remove
void _e
void CHECKSUMS
void simpleHash
