import { useMemo } from 'react'

interface PunkAvatarProps {
  seed: string
  size?: number
  className?: string
}

// Hash function to generate consistent random values from seed
function hashCode(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i)
    hash = ((hash << 5) - hash) + char
    hash = hash & hash
  }
  return Math.abs(hash)
}

// Get a value from hash at specific position
function getHashValue(hash: number, position: number, max: number): number {
  return ((hash >> (position * 4)) & 0xF) % max
}

// Color palettes - Web3/Crypto aesthetic
const BACKGROUNDS = [
  '#1a1a2e', '#16213e', '#0f3460', '#1b1b2f', '#162447',
  '#1f1f3d', '#2d132c', '#1e1e3f', '#0d1b2a', '#1b263b',
  '#252538', '#2a2a4a', '#1e2a3a', '#0f172a', '#1a1f35',
]

const SKIN_TONES = [
  '#ffd5c8', '#f5c5b5', '#daa06d', '#c68642', '#8d5524',
  '#6b4423', '#4a3728', '#ffdbac', '#f1c27d', '#e0ac69',
]

const HAIR_COLORS = [
  '#090806', '#2c222b', '#3b3024', '#4a4035', '#504444',
  '#6a4e42', '#a55728', '#b55239', '#8d4a43', '#91553d',
  '#e6cea8', '#e5c8a8', '#debc99', '#977961', '#343434',
  '#9a3300', '#ff6b6b', '#4ecdc4', '#ffe66d', '#a855f7',
]

const ACCESSORY_COLORS = [
  '#F0B90B', '#0ECB81', '#F6465D', '#60a5fa', '#a855f7',
  '#ec4899', '#14b8a6', '#f97316', '#84cc16', '#06b6d4',
]

export function PunkAvatar({ seed, size = 40, className = '' }: PunkAvatarProps) {
  const avatar = useMemo(() => {
    const hash = hashCode(seed)

    // Deterministic selections based on hash
    const bgColor = BACKGROUNDS[getHashValue(hash, 0, BACKGROUNDS.length)]
    const skinColor = SKIN_TONES[getHashValue(hash, 1, SKIN_TONES.length)]
    const hairColor = HAIR_COLORS[getHashValue(hash, 2, HAIR_COLORS.length)]
    const accColor = ACCESSORY_COLORS[getHashValue(hash, 3, ACCESSORY_COLORS.length)]

    const hairStyle = getHashValue(hash, 4, 8)
    const eyeStyle = getHashValue(hash, 5, 6)
    const mouthStyle = getHashValue(hash, 6, 5)
    const hasGlasses = getHashValue(hash, 7, 4) === 0
    const hasEarring = getHashValue(hash, 8, 5) === 0
    const hasMask = getHashValue(hash, 9, 8) === 0
    const hasLaser = getHashValue(hash, 10, 12) === 0

    return {
      bgColor,
      skinColor,
      hairColor,
      accColor,
      hairStyle,
      eyeStyle,
      mouthStyle,
      hasGlasses,
      hasEarring,
      hasMask,
      hasLaser,
    }
  }, [seed])

  // Pixel size for 24x24 grid
  const px = size / 24

  const renderHair = () => {
    const { hairColor, hairStyle } = avatar
    switch (hairStyle) {
      case 0: // Mohawk
        return (
          <>
            <rect x={11*px} y={2*px} width={2*px} height={5*px} fill={hairColor} />
            <rect x={10*px} y={3*px} width={4*px} height={1*px} fill={hairColor} />
          </>
        )
      case 1: // Messy
        return (
          <>
            <rect x={7*px} y={4*px} width={10*px} height={3*px} fill={hairColor} />
            <rect x={8*px} y={3*px} width={8*px} height={1*px} fill={hairColor} />
            <rect x={6*px} y={5*px} width={2*px} height={2*px} fill={hairColor} />
            <rect x={16*px} y={5*px} width={2*px} height={2*px} fill={hairColor} />
          </>
        )
      case 2: // Cap
        return (
          <>
            <rect x={6*px} y={5*px} width={12*px} height={3*px} fill={avatar.accColor} />
            <rect x={5*px} y={7*px} width={14*px} height={1*px} fill={avatar.accColor} />
            <rect x={7*px} y={4*px} width={10*px} height={1*px} fill={avatar.accColor} />
          </>
        )
      case 3: // Long
        return (
          <>
            <rect x={7*px} y={4*px} width={10*px} height={4*px} fill={hairColor} />
            <rect x={6*px} y={6*px} width={2*px} height={8*px} fill={hairColor} />
            <rect x={16*px} y={6*px} width={2*px} height={8*px} fill={hairColor} />
          </>
        )
      case 4: // Bald with shine
        return (
          <rect x={9*px} y={5*px} width={2*px} height={1*px} fill="rgba(255,255,255,0.3)" />
        )
      case 5: // Spiky
        return (
          <>
            <rect x={7*px} y={5*px} width={10*px} height={2*px} fill={hairColor} />
            <rect x={8*px} y={3*px} width={2*px} height={2*px} fill={hairColor} />
            <rect x={11*px} y={2*px} width={2*px} height={3*px} fill={hairColor} />
            <rect x={14*px} y={3*px} width={2*px} height={2*px} fill={hairColor} />
          </>
        )
      case 6: // Hoodie
        return (
          <>
            <rect x={5*px} y={6*px} width={14*px} height={6*px} fill={avatar.accColor} />
            <rect x={6*px} y={5*px} width={12*px} height={1*px} fill={avatar.accColor} />
            <rect x={8*px} y={8*px} width={8*px} height={4*px} fill={avatar.skinColor} />
          </>
        )
      case 7: // Crown
        return (
          <>
            <rect x={7*px} y={4*px} width={10*px} height={1*px} fill="#F0B90B" />
            <rect x={8*px} y={2*px} width={2*px} height={2*px} fill="#F0B90B" />
            <rect x={11*px} y={1*px} width={2*px} height={3*px} fill="#F0B90B" />
            <rect x={14*px} y={2*px} width={2*px} height={2*px} fill="#F0B90B" />
          </>
        )
      default:
        return null
    }
  }

  const renderEyes = () => {
    const { eyeStyle, accColor } = avatar
    const eyeY = 10 * px

    switch (eyeStyle) {
      case 0: // Normal
        return (
          <>
            <rect x={8*px} y={eyeY} width={2*px} height={2*px} fill="#000" />
            <rect x={14*px} y={eyeY} width={2*px} height={2*px} fill="#000" />
            <rect x={8*px} y={eyeY} width={1*px} height={1*px} fill="#fff" />
            <rect x={14*px} y={eyeY} width={1*px} height={1*px} fill="#fff" />
          </>
        )
      case 1: // Angry
        return (
          <>
            <rect x={8*px} y={eyeY} width={2*px} height={2*px} fill="#000" />
            <rect x={14*px} y={eyeY} width={2*px} height={2*px} fill="#000" />
            <rect x={7*px} y={9*px} width={3*px} height={1*px} fill={avatar.skinColor} />
            <rect x={14*px} y={9*px} width={3*px} height={1*px} fill={avatar.skinColor} />
          </>
        )
      case 2: // Wink
        return (
          <>
            <rect x={8*px} y={eyeY} width={2*px} height={2*px} fill="#000" />
            <rect x={14*px} y={10.5*px} width={2*px} height={1*px} fill="#000" />
          </>
        )
      case 3: // Sleepy
        return (
          <>
            <rect x={8*px} y={10.5*px} width={2*px} height={1*px} fill="#000" />
            <rect x={14*px} y={10.5*px} width={2*px} height={1*px} fill="#000" />
          </>
        )
      case 4: // Big eyes
        return (
          <>
            <rect x={7*px} y={9*px} width={3*px} height={3*px} fill="#fff" />
            <rect x={14*px} y={9*px} width={3*px} height={3*px} fill="#fff" />
            <rect x={8*px} y={10*px} width={2*px} height={2*px} fill="#000" />
            <rect x={15*px} y={10*px} width={2*px} height={2*px} fill="#000" />
          </>
        )
      case 5: // Robot
        return (
          <>
            <rect x={7*px} y={9*px} width={3*px} height={3*px} fill={accColor} />
            <rect x={14*px} y={9*px} width={3*px} height={3*px} fill={accColor} />
            <rect x={8*px} y={10*px} width={1*px} height={1*px} fill="#000" />
            <rect x={15*px} y={10*px} width={1*px} height={1*px} fill="#000" />
          </>
        )
      default:
        return null
    }
  }

  const renderMouth = () => {
    const { mouthStyle } = avatar
    const mouthY = 14 * px

    switch (mouthStyle) {
      case 0: // Smile
        return (
          <>
            <rect x={10*px} y={mouthY} width={4*px} height={1*px} fill="#000" />
            <rect x={9*px} y={13*px} width={1*px} height={1*px} fill="#000" />
            <rect x={14*px} y={13*px} width={1*px} height={1*px} fill="#000" />
          </>
        )
      case 1: // Neutral
        return <rect x={10*px} y={mouthY} width={4*px} height={1*px} fill="#000" />
      case 2: // Smirk
        return (
          <>
            <rect x={11*px} y={mouthY} width={3*px} height={1*px} fill="#000" />
            <rect x={14*px} y={13*px} width={1*px} height={1*px} fill="#000" />
          </>
        )
      case 3: // Open
        return (
          <>
            <rect x={10*px} y={13*px} width={4*px} height={2*px} fill="#000" />
            <rect x={11*px} y={14*px} width={2*px} height={1*px} fill="#ff6b6b" />
          </>
        )
      case 4: // Teeth
        return (
          <>
            <rect x={10*px} y={mouthY} width={4*px} height={2*px} fill="#000" />
            <rect x={10*px} y={mouthY} width={4*px} height={1*px} fill="#fff" />
          </>
        )
      default:
        return null
    }
  }

  const renderAccessories = () => {
    const { hasGlasses, hasEarring, hasMask, hasLaser, accColor } = avatar
    const elements = []

    if (hasGlasses) {
      elements.push(
        <g key="glasses">
          <rect x={6*px} y={9*px} width={5*px} height={4*px} fill="transparent" stroke={accColor} strokeWidth={px} />
          <rect x={13*px} y={9*px} width={5*px} height={4*px} fill="transparent" stroke={accColor} strokeWidth={px} />
          <rect x={11*px} y={10*px} width={2*px} height={1*px} fill={accColor} />
        </g>
      )
    }

    if (hasEarring) {
      elements.push(
        <circle key="earring" cx={5*px} cy={12*px} r={px} fill="#F0B90B" />
      )
    }

    if (hasMask) {
      elements.push(
        <g key="mask">
          <rect x={7*px} y={13*px} width={10*px} height={4*px} fill="#1a1a2e" />
          <rect x={8*px} y={14*px} width={2*px} height={1*px} fill={accColor} />
          <rect x={14*px} y={14*px} width={2*px} height={1*px} fill={accColor} />
        </g>
      )
    }

    if (hasLaser) {
      elements.push(
        <g key="laser">
          <rect x={9*px} y={10*px} width={15*px} height={2*px} fill="#F6465D" opacity={0.8} />
          <rect x={10*px} y={10.5*px} width={14*px} height={1*px} fill="#fff" opacity={0.5} />
        </g>
      )
    }

    return elements
  }

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      className={className}
      style={{ imageRendering: 'pixelated' }}
    >
      {/* Background */}
      <rect width={size} height={size} fill={avatar.bgColor} rx={size * 0.15} />

      {/* Head shape */}
      <rect x={7*px} y={6*px} width={10*px} height={12*px} fill={avatar.skinColor} />
      <rect x={8*px} y={5*px} width={8*px} height={1*px} fill={avatar.skinColor} />
      <rect x={8*px} y={18*px} width={8*px} height={1*px} fill={avatar.skinColor} />

      {/* Ears */}
      <rect x={6*px} y={10*px} width={1*px} height={3*px} fill={avatar.skinColor} />
      <rect x={17*px} y={10*px} width={1*px} height={3*px} fill={avatar.skinColor} />

      {/* Neck */}
      <rect x={10*px} y={18*px} width={4*px} height={3*px} fill={avatar.skinColor} />

      {/* Hair (rendered before accessories) */}
      {renderHair()}

      {/* Eyes */}
      {renderEyes()}

      {/* Nose */}
      <rect x={11*px} y={12*px} width={2*px} height={1*px} fill={avatar.skinColor} style={{ filter: 'brightness(0.9)' }} />

      {/* Mouth */}
      {renderMouth()}

      {/* Accessories (glasses, earrings, etc.) */}
      {renderAccessories()}
    </svg>
  )
}

// Pre-defined punk collection for special traders
export function getTraderAvatar(traderId: string, traderName: string): string {
  // Use a combination of ID and name for more unique results
  return `${traderId}-${traderName}`
}
