/// <reference types="vite/client" />

import { useState } from 'react'
import { confirmToast, notify } from '../lib/notify'

const toastOptions = [
  'message',
  'success',
  'info',
  'warning',
  'error',
  'custom',
] as const

type ToastType = (typeof toastOptions)[number]

const customRenderer = () => (
  <div className="dev-custom-toast">
    <p className="dev-custom-title">Sonner 自定义通知</p>
    <p className="dev-custom-body">
      这是一个通过 `notify.custom` 渲染的测试 Toast
    </p>
  </div>
)

export function DevToastController() {
  const [type, setType] = useState<ToastType>('success')
  const [message, setMessage] = useState('来自 Dev 控制器的测试通知')
  const [duration, setDuration] = useState(2200)

  if (!import.meta.env.DEV) {
    return null
  }

  const triggerToast = async () => {
    switch (type) {
      case 'message':
        notify.message(message, { duration })
        break
      case 'success':
        notify.success(message, { duration })
        break
      case 'info':
        notify.info(message, { duration })
        break
      case 'warning':
        notify.warning(message, { duration })
        break
      case 'error':
        notify.error(message, { duration })
        break
      case 'custom':
        notify.custom(() => customRenderer(), { duration })
        break
    }
  }

  const triggerConfirm = async () => {
    const confirmed = await confirmToast(message, {
      okText: '继续',
      cancelText: '取消',
    })
    if (confirmed) {
      notify.success('确认按钮已点击', { duration: 2000 })
    } else {
      notify.message('已取消确认逻辑', { duration: 2000 })
    }
  }

  return (
    <div className="dev-toast-controller">
      <div className="dev-toast-controller__header">
        <span>Dev Sonner 控制器</span>
        <small>仅在 dev 模式可见</small>
      </div>
      <div className="dev-toast-controller__content">
        <label className="dev-toast-controller__label">
          类型
          <select
            value={type}
            onChange={(event) => setType(event.target.value as ToastType)}
          >
            {toastOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
        <label className="dev-toast-controller__label">
          文案
          <input
            value={message}
            onChange={(event) => setMessage(event.target.value)}
            placeholder="输入通知/确认文案"
          />
        </label>
        <label className="dev-toast-controller__label">
          持续(ms)
          <input
            type="number"
            min={600}
            value={duration}
            onChange={(event) => setDuration(Number(event.target.value))}
          />
        </label>
        <div className="dev-toast-controller__actions">
          <button onClick={triggerToast}>触发通知</button>
          <button onClick={triggerConfirm}>触发确认</button>
        </div>
      </div>
    </div>
  )
}

export default DevToastController
