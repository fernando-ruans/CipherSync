import {useEffect, useState} from 'react'
import {Copy} from 'lucide-react'
import {localTOTP} from '../lib/totp'
import {safeCopy} from '../lib/util'
import {useT} from '../lib/locales'
import {useApp} from '../state'

function CountdownRing({seconds, total = 30}: {seconds: number; total?: number}) {
    const radius = 26
    const circumference = 2 * Math.PI * radius
    const progress = Math.max(0, Math.min(1, seconds / total))
    const danger = seconds <= 10
    return (
        <svg width="64" height="64" viewBox="0 0 64 64" className="shrink-0">
            <circle cx="32" cy="32" r={radius} fill="none" stroke="var(--edge)" strokeWidth="4"/>
            <circle
                cx="32"
                cy="32"
                r={radius}
                fill="none"
                stroke={danger ? '#f59e0b' : 'var(--accent)'}
                strokeWidth="4"
                strokeLinecap="round"
                strokeDasharray={circumference}
                strokeDashoffset={circumference * (1 - progress)}
                transform="rotate(-90 32 32)"
                style={{transition: 'stroke-dashoffset 1s linear'}}
            />
        </svg>
    )
}

export function TOTPDisplay({itemId}: {itemId: string}) {
    const t = useT()
    const secret = useApp((s) => s.items.find((i) => i.id === itemId)?.totpSecret ?? '')
    const [code, setCode] = useState('')
    const [seconds, setSeconds] = useState(0)
    const [error, setError] = useState(false)

    useEffect(() => {
        let alive = true
        const tick = async () => {
            if (!secret) {
                if (alive) setError(true)
                return
            }
            try {
                const c = await localTOTP(secret)
                if (alive) {
                    setCode(c.code)
                    setSeconds(c.remaining)
                    setError(false)
                }
            } catch {
                if (alive) setError(true)
            }
        }
        void tick()
        const id = setInterval(tick, 1000)
        return () => {
            alive = false
            clearInterval(id)
        }
    }, [secret])

    if (error || !secret) return null

    async function copy() {
        await safeCopy(code, t('totp.codeCopied'))
    }

    return (
        <div className="flex items-center gap-4 rounded-xl border border-edge bg-input p-4">
            <CountdownRing seconds={seconds}/>
            <div className="min-w-0 flex-1">
                <div className="text-xs font-medium text-mut">{t('totp.code')}</div>
                <div className="mt-0.5 flex items-center gap-3">
                    <span className="font-mono text-3xl font-bold tracking-[0.2em] text-ink">{code || '······'}</span>
                    <button
                        onClick={() => void copy()}
                        title={t('totp.copyCode')}
                        className="rounded-lg p-1.5 text-mut transition-colors hover:bg-hover hover:text-ink"
                    >
                        <Copy size={16}/>
                    </button>
                </div>
            </div>
        </div>
    )
}

// useTOTPCode computes the current code locally for inline displays.
// Instead of ticking every second it re-schedules for the next TOTP period:
// with many rows that means ~1 computation/30s per item instead of 1/s.
export function useTOTPCode(secret: string | undefined) {
    const [state, setState] = useState({code: '', remaining: 0})
    useEffect(() => {
        if (!secret) return
        let alive = true
        let timer: number | undefined
        const tick = async () => {
            try {
                const c = await localTOTP(secret)
                if (!alive) return
                setState((prev) => {
                    if (prev.code === c.code && prev.remaining === c.remaining) return prev
                    return {code: c.code, remaining: c.remaining}
                })
                timer = window.setTimeout(tick, Math.max(1000, c.remaining * 1000))
            } catch {
                // ignore
            }
        }
        void tick()
        return () => {
            alive = false
            if (timer) clearTimeout(timer)
        }
    }, [secret])
    return state
}
