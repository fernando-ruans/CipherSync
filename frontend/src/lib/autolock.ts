import {useEffect, useRef} from 'react'
import {useApp} from '../state'

export function useAutoLock(minutes: number) {
    const lastActivity = useRef(Date.now())
    const lock = useApp((s) => s.lock)

    useEffect(() => {
        if (minutes <= 0) return
        const handlers = ['mousemove', 'mousedown', 'keydown', 'scroll', 'touchstart'] as const
        const bump = () => {
            lastActivity.current = Date.now()
        }
        handlers.forEach((h) => window.addEventListener(h, bump, {passive: true}))

        const id = window.setInterval(() => {
            if (Date.now() - lastActivity.current > minutes * 60 * 1000) {
                void lock()
            }
        }, 15000)

        const onVisibility = () => {
            if (document.hidden) void lock()
        }
        document.addEventListener('visibilitychange', onVisibility)

        return () => {
            handlers.forEach((h) => window.removeEventListener(h, bump))
            window.clearInterval(id)
            document.removeEventListener('visibilitychange', onVisibility)
        }
    }, [minutes, lock])
}
