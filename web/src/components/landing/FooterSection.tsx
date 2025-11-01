import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { getExchangeIcon } from '../ExchangeIcons'

export default function FooterSection() {
  const { language } = useLanguage()
  return (
    <footer style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}>
      <div className="max-w-[1200px] mx-auto px-6 py-10">
        {/* Multi-link columns */}
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-8">
          <div>
            <h3 className="text-sm font-semibold mb-3" style={{ color: '#EAECEF' }}>链接</h3>
            <ul className="space-y-2 text-sm" style={{ color: '#848E9C' }}>
              <li><a className="hover:text-[#F0B90B]" href="https://github.com/tinkle-community/nofx" target="_blank" rel="noopener noreferrer">GitHub</a></li>
              <li><a className="hover:text-[#F0B90B]" href="https://t.me/nofx_dev_community" target="_blank" rel="noopener noreferrer">Telegram</a></li>
              <li><a className="hover:text-[#F0B90B]" href="https://x.com/nofx_ai" target="_blank" rel="noopener noreferrer">X (Twitter)</a></li>
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold mb-3" style={{ color: '#EAECEF' }}>资源</h3>
            <ul className="space-y-2 text-sm" style={{ color: '#848E9C' }}>
              <li><a className="hover:text-[#F0B90B]" href="/README.zh-CN.md" target="_blank" rel="noopener noreferrer">文档</a></li>
              <li><a className="hover:text-[#F0B90B]" href="/DOCKER_DEPLOY.md" target="_blank" rel="noopener noreferrer">Docker 部署</a></li>
              <li><a className="hover:text-[#F0B90B]" href="/PM2_DEPLOYMENT.md" target="_blank" rel="noopener noreferrer">PM2 部署</a></li>
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold mb-3" style={{ color: '#EAECEF' }}>产品</h3>
            <ul className="space-y-2 text-sm" style={{ color: '#848E9C' }}>
              <li><a className="hover:text-[#F0B90B]" href="#how-it-works">如何开始</a></li>
              <li><a className="hover:text-[#F0B90B]" href="#features">核心功能</a></li>
              <li><a className="hover:text-[#F0B90B]" href="#">开源生态</a></li>
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold mb-3" style={{ color: '#EAECEF' }}>支持的交易所</h3>
            <ul className="space-y-3 text-sm" style={{ color: '#848E9C' }}>
              <li>
                <a className="hover:text-[#F0B90B] inline-flex items-center gap-2" href="https://www.binance.com/" target="_blank" rel="noopener noreferrer">
                  <span className="inline-flex items-center" style={{ width: 20, height: 20 }}>
                    {getExchangeIcon('binance', { width: 20, height: 20 })}
                  </span>
                  Binance
                </a>
              </li>
              <li>
                <a className="hover:text-[#F0B90B] inline-flex items-center gap-2" href="https://aster.network/" target="_blank" rel="noopener noreferrer">
                  <span className="inline-flex items-center" style={{ width: 20, height: 20 }}>
                    {getExchangeIcon('aster', { width: 20, height: 20 })}
                  </span>
                  Aster DEX
                </a>
              </li>
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold mb-3" style={{ color: '#EAECEF' }}>支持</h3>
            <ul className="space-y-2 text-sm" style={{ color: '#848E9C' }}>
              <li><a className="hover:text-[#F0B90B]" href="/README.zh-CN.md#%E5%B8%B8%E8%A7%81%E9%97%AE%E9%A2%98" target="_blank" rel="noopener noreferrer">常见问题</a></li>
              <li><a className="hover:text-[#F0B90B]" href="https://github.com/tinkle-community/nofx/issues" target="_blank" rel="noopener noreferrer">报告问题</a></li>
              <li><a className="hover:text-[#F0B90B]" href="/README.md#contributing" target="_blank" rel="noopener noreferrer">贡献指南</a></li>
            </ul>
          </div>
        </div>

        {/* Bottom note */}
        <div className="pt-8 mt-8 text-center text-xs" style={{ color: '#5E6673', borderTop: '1px solid #2B3139' }}>
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </div>
      </div>
    </footer>
  )
}
