import React from 'react'

interface IconProps {
  width?: number
  height?: number
  className?: string
}

// Binance SVG 图标组件
const BinanceIcon: React.FC<IconProps> = ({
  width = 24,
  height = 24,
  className,
}) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width={width}
    height={height}
    viewBox="-52.785 -88 457.47 528"
    className={className}
  >
    <path
      d="M79.5 176l-39.7 39.7L0 176l39.7-39.7zM176 79.5l68.1 68.1 39.7-39.7L176 0 68.1 107.9l39.7 39.7zm136.2 56.8L272.5 176l39.7 39.7 39.7-39.7zM176 272.5l-68.1-68.1-39.7 39.7L176 352l107.8-107.9-39.7-39.7zm0-56.8l39.7-39.7-39.7-39.7-39.8 39.7z"
      fill="#f0b90b"
    />
  </svg>
)

// Hyperliquid SVG 图标组件
const HyperliquidIcon: React.FC<IconProps> = ({
  width = 24,
  height = 24,
  className,
}) => (
  <svg
    width={width}
    height={height}
    viewBox="0 0 144 144"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path
      d="M144 71.6991C144 119.306 114.866 134.582 99.5156 120.98C86.8804 109.889 83.1211 86.4521 64.116 84.0456C39.9942 81.0113 37.9057 113.133 22.0334 113.133C3.5504 113.133 0 86.2428 0 72.4315C0 58.3063 3.96809 39.0542 19.736 39.0542C38.1146 39.0542 39.1588 66.5722 62.132 65.1073C85.0007 63.5379 85.4184 34.8689 100.247 22.6271C113.195 12.0593 144 23.4641 144 71.6991Z"
      fill="#97FCE4"
    />
  </svg>
)

// Bybit SVG 图标组件 (Official from bybit-web3.github.io)
const BybitIcon: React.FC<IconProps> = ({
  width = 24,
  height = 24,
  className,
}) => (
  <svg
    width={width}
    height={height}
    viewBox="0 0 88 88"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M0 18.7C0 8.37227 8.37228 0 18.7 0H69.3C79.6277 0 88 8.37228 88 18.7V69.3C88 79.6277 79.6277 88 69.3 88H18.7C8.37227 88 0 79.6277 0 69.3V18.7Z" fill="#404347"/>
    <path d="M7.57617 26.8067C6.78516 24.0787 8.4775 21.2531 11.2559 20.663L57.6087 10.8173C59.809 10.35 62.0443 11.4443 63.0247 13.4689L83.8443 56.4657L25.1776 87.5101L7.57617 26.8067Z" fill="url(#bybit_grad)"/>
    <path d="M8.18242 30.1618C7.35049 27.2838 9.27925 24.3413 12.2502 23.9559L73.6865 15.9881C76.2391 15.6571 78.6111 17.3618 79.1111 19.8867L88.0003 64.7771L24.6892 87.2665L8.18242 30.1618Z" fill="white"/>
    <path d="M0 34.2222C0 28.8221 4.37766 24.4445 9.77778 24.4445H68.4444C79.2447 24.4445 88 33.1998 88 44V68.4445C88 79.2447 79.2447 88 68.4444 88H19.5556C8.75532 88 0 79.2447 0 68.4445V34.2222Z" fill="black"/>
    <path d="M58.2201 61.1959V42.8755H61.7937V61.1959H58.2201Z" fill="#F7A600"/>
    <path d="M17.4395 66.6637H9.77795V48.3434H17.1313C20.7049 48.3434 22.7874 50.3505 22.7874 53.4893C22.7874 55.5215 21.4504 56.8345 20.5257 57.2721C21.6315 57.7869 23.0456 58.9438 23.0456 61.3885C23.0456 64.8108 20.7049 66.6637 17.4395 66.6637ZM16.8481 51.5343H13.3516V55.7548H16.8481C18.3642 55.7548 19.2138 54.9064 19.2138 53.6455C19.2138 52.3826 18.3662 51.5343 16.8481 51.5343ZM17.0793 58.9708H13.3516V63.4728H17.0793C18.6994 63.4728 19.47 62.4432 19.47 61.2092C19.472 59.9733 18.6994 58.9708 17.0793 58.9708Z" fill="white"/>
    <path d="M32.8925 59.1501V66.6637H29.3439V59.1501L23.8419 48.3434H27.7238L31.1432 55.7278L34.5107 48.3434H38.3926L32.8925 59.1501Z" fill="white"/>
    <path d="M48.5633 66.6637H40.9017V48.3434H48.2551C51.8287 48.3434 53.9112 50.3505 53.9112 53.4893C53.9112 55.5215 52.5742 56.8345 51.6495 57.2721C52.7553 57.7869 54.1693 58.9438 54.1693 61.3885C54.1674 64.8108 51.8268 66.6637 48.5633 66.6637ZM47.9719 51.5343H44.4753V55.7548H47.9719C49.488 55.7548 50.3376 54.9064 50.3376 53.6455C50.3357 52.3826 49.488 51.5343 47.9719 51.5343ZM48.2031 58.9708H44.4753V63.4728H48.2031C49.8232 63.4728 50.5938 62.4432 50.5938 61.2092C50.5938 59.9734 49.8213 58.9708 48.2031 58.9708Z" fill="white"/>
    <path d="M73.439 51.5343V66.6637H69.8654V51.5343H65.0839V48.3434H78.2224V51.5343H73.439Z" fill="white"/>
    <defs>
      <linearGradient id="bybit_grad" x1="7.33308" y1="25.594" x2="84.6381" y2="21.7216" gradientUnits="userSpaceOnUse">
        <stop stopColor="#FFD748"/>
        <stop offset="1" stopColor="#F7A600"/>
      </linearGradient>
    </defs>
  </svg>
)

// OKX SVG 图标组件
const OKXIcon: React.FC<IconProps> = ({
  width = 24,
  height = 24,
  className,
}) => (
  <svg
    width={width}
    height={height}
    viewBox="0 0 200 200"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <rect width="200" height="200" rx="40" fill="#000"/>
    <rect x="35" y="35" width="40" height="40" rx="4" fill="#fff"/>
    <rect x="80" y="80" width="40" height="40" rx="4" fill="#fff"/>
    <rect x="125" y="35" width="40" height="40" rx="4" fill="#fff"/>
    <rect x="35" y="125" width="40" height="40" rx="4" fill="#fff"/>
    <rect x="125" y="125" width="40" height="40" rx="4" fill="#fff"/>
  </svg>
)

// Bitget SVG 图标组件
const BitgetIcon: React.FC<IconProps> = ({
  width = 24,
  height = 24,
  className,
}) => (
  <svg
    width={width}
    height={height}
    viewBox="0 0 200 200"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <rect width="200" height="200" rx="40" fill="#00F0FF"/>
    <path d="M60 45L100 45C127.614 45 150 67.3858 150 95C150 122.614 127.614 145 100 145L80 145L80 155L60 155L60 45ZM80 65L80 125L100 125C116.569 125 130 111.569 130 95C130 78.4315 116.569 65 100 65L80 65Z" fill="#000"/>
    <path d="M75 80L75 110L95 110C103.284 110 110 103.284 110 95C110 86.7157 103.284 80 95 80L75 80Z" fill="#00F0FF"/>
  </svg>
)

// Aster SVG 图标组件
const AsterIcon: React.FC<IconProps> = ({
  width = 24,
  height = 24,
  className,
}) => (
  <svg
    width={width}
    height={height}
    viewBox="0 0 32 32"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <defs>
      <linearGradient
        id="paint0_linear_428_3535"
        x1="18.9416"
        y1="4.14314e-07"
        x2="12.6408"
        y2="32.0507"
        gradientUnits="userSpaceOnUse"
      >
        <stop stopColor="#F4D5B1" />
        <stop offset="1" stopColor="#FFD29F" />
      </linearGradient>
      <linearGradient
        id="paint1_linear_428_3535"
        x1="18.9416"
        y1="4.14314e-07"
        x2="12.6408"
        y2="32.0507"
        gradientUnits="userSpaceOnUse"
      >
        <stop stopColor="#F4D5B1" />
        <stop offset="1" stopColor="#FFD29F" />
      </linearGradient>
      <linearGradient
        id="paint2_linear_428_3535"
        x1="18.9416"
        y1="4.14314e-07"
        x2="12.6408"
        y2="32.0507"
        gradientUnits="userSpaceOnUse"
      >
        <stop stopColor="#F4D5B1" />
        <stop offset="1" stopColor="#FFD29F" />
      </linearGradient>
      <linearGradient
        id="paint3_linear_428_3535"
        x1="18.9416"
        y1="4.14314e-07"
        x2="12.6408"
        y2="32.0507"
        gradientUnits="userSpaceOnUse"
      >
        <stop stopColor="#F4D5B1" />
      </linearGradient>
    </defs>
    <path
      d="M9.13309 30.4398L9.88315 26.9871C10.7197 23.1362 7.77521 19.4988 3.82118 19.4988H0.385363C1.4689 24.3374 4.75127 28.3496 9.13309 30.4398Z"
      fill="url(#paint0_linear_428_3535)"
    />
    <path
      d="M10.64 31.0663C12.3326 31.6707 14.1567 32 16.0579 32C23.7199 32 30.1285 26.6527 31.7305 19.4988H21.249C16.5244 19.4988 12.4396 22.7824 11.44 27.3838L10.64 31.0663Z"
      fill="url(#paint1_linear_428_3535)"
    />
    <path
      d="M32.0038 17.8987C32.0778 17.2756 32.1159 16.6415 32.1159 15.9985C32.1159 7.60402 25.629 0.719287 17.3779 0.0503251L15.1273 10.4105C14.2907 14.2614 17.2352 17.8987 21.1892 17.8987H32.0038Z"
      fill="url(#paint2_linear_428_3535)"
    />
    <path
      d="M15.7459 0C7.02134 0.165717 0 7.26504 0 15.9985C0 16.6415 0.0380539 17.2756 0.112041 17.8987H3.76146C8.48603 17.8987 12.5709 14.6151 13.5705 10.0137L15.7459 0Z"
      fill="url(#paint3_linear_428_3535)"
    />
  </svg>
)

// Lighter SVG 图标组件
const LighterIcon: React.FC<IconProps> = ({
  width = 24,
  height = 24,
  className,
}) => (
  <svg
    width={width}
    height={height}
    viewBox="0 0 200 200"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <rect width="200" height="200" rx="40" fill="#1A1A2E"/>
    <path d="M70 50L70 130L130 130L130 150L50 150L50 50L70 50Z" fill="#00D9FF"/>
    <circle cx="115" cy="65" r="20" fill="#00D9FF"/>
  </svg>
)

// 获取交易所图标的函数
export const getExchangeIcon = (
  exchangeType: string,
  props: IconProps = {}
) => {
  // 支持完整ID或类型名
  const type = exchangeType.toLowerCase().includes('binance')
    ? 'binance'
    : exchangeType.toLowerCase().includes('bybit')
      ? 'bybit'
      : exchangeType.toLowerCase().includes('okx')
        ? 'okx'
        : exchangeType.toLowerCase().includes('bitget')
          ? 'bitget'
          : exchangeType.toLowerCase().includes('hyperliquid')
            ? 'hyperliquid'
            : exchangeType.toLowerCase().includes('aster')
              ? 'aster'
              : exchangeType.toLowerCase().includes('lighter')
                ? 'lighter'
                : exchangeType.toLowerCase()

  const iconProps = {
    width: props.width || 24,
    height: props.height || 24,
    className: props.className,
  }

  switch (type) {
    case 'binance':
      return <BinanceIcon {...iconProps} />
    case 'bybit':
      return <BybitIcon {...iconProps} />
    case 'okx':
      return <OKXIcon {...iconProps} />
    case 'bitget':
      return <BitgetIcon {...iconProps} />
    case 'hyperliquid':
    case 'dex':
      return <HyperliquidIcon {...iconProps} />
    case 'aster':
      return <AsterIcon {...iconProps} />
    case 'lighter':
      return <LighterIcon {...iconProps} />
    case 'cex':
    default:
      return (
        <div
          className={props.className}
          style={{
            width: props.width || 24,
            height: props.height || 24,
            borderRadius: '50%',
            background: '#2B3139',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '12px',
            fontWeight: 'bold',
            color: '#EAECEF',
          }}
        >
          {type[0]?.toUpperCase() || '?'}
        </div>
      )
  }
}
