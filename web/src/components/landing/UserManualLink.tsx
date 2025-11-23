import { motion } from 'framer-motion'
import { BookText } from 'lucide-react'

export default function UserManualLink() {
  return (
    <div className='flex justify-center my-12'>
      <motion.a
        href='/docs/user-manual'
        className='flex items-center gap-2 px-8 py-3 rounded-lg font-semibold'
        style={{
          background: 'rgba(240, 185, 11, 0.1)',
          color: 'var(--brand-yellow)',
          border: '2px solid var(--brand-yellow)'
        }}
        whileHover={{ scale: 1.05 }}
        whileTap={{ scale: 0.95 }}
      >
        <BookText className='w-5 h-5' />
        User Manual
      </motion.a>
    </div>
  )
}
