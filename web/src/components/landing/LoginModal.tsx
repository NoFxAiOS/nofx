import { motion } from 'framer-motion'
import { t, Language } from '../../i18n/translations'
import { Modal } from '../ui/Modal'

interface LoginModalProps {
  onClose: () => void
  language: Language
}

export default function LoginModal({ onClose, language }: LoginModalProps) {
  return (
    <Modal open={true} onOpenChange={(open) => !open && onClose()}>
      <Modal.Content className="max-w-md">
        <div className="p-8">
          <h2
            className="text-2xl font-bold mb-6"
            style={{ color: 'var(--brand-light-gray)' }}
          >
            {t('accessNofxPlatform', language)}
          </h2>
          <p
            className="text-sm mb-6"
            style={{ color: 'var(--text-secondary)' }}
          >
            {t('loginRegisterPrompt', language)}
          </p>
          <div className="space-y-3">
            <motion.button
              onClick={() => {
                window.history.pushState({}, '', '/login')
                window.dispatchEvent(new PopStateEvent('popstate'))
                onClose()
              }}
              className="block w-full px-6 py-3 rounded-lg font-semibold text-center"
              style={{
                background: 'var(--brand-yellow)',
                color: 'var(--brand-black)',
              }}
              whileHover={{
                scale: 1.05,
                boxShadow: '0 10px 30px rgba(240, 185, 11, 0.4)',
              }}
              whileTap={{ scale: 0.95 }}
            >
              {t('signIn', language)}
            </motion.button>
            <motion.button
              onClick={() => {
                window.history.pushState({}, '', '/register')
                window.dispatchEvent(new PopStateEvent('popstate'))
                onClose()
              }}
              className="block w-full px-6 py-3 rounded-lg font-semibold text-center"
              style={{
                background: 'var(--brand-dark-gray)',
                color: 'var(--brand-light-gray)',
                border: '1px solid rgba(240, 185, 11, 0.2)',
              }}
              whileHover={{ scale: 1.05, borderColor: 'var(--brand-yellow)' }}
              whileTap={{ scale: 0.95 }}
            >
              {t('registerNewAccount', language)}
            </motion.button>
          </div>
        </div>
      </Modal.Content>
    </Modal>
  )
}
