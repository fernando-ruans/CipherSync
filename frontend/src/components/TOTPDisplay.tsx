import {useEffect, useState} from 'react'
import toast from 'react-hot-toast'
import {Copy} from 'lucide-react'
import {api} from '../lib/api'

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
    const [code, setCode] = useState('')
    const [seconds, setSeconds] = useState(0)
    const [error, setError] = useState(false)

    useEffect(() => {
        let alive = true
        const tick = async () => {
            try {
                const c = await api.getTOTPCode(itemId)
                if (alive) {
                    setCode(c.code)
                    setSeconds(c.secondsRemaining)
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
    }, [itemId])

    if (error) return null

    async function copy() {
        await api.copy(code)
        toast.success('Código copiado')
    }

    return (
        <div className="flex items-center gap-4 rounded-xl border border-edge bg-input p-4">
            <CountdownRing seconds={seconds}/>
            <div className="min-w-0 flex-1">
                <div className="text-xs font-medium text-mut">Código de verificação (2FA)</div>
                <div className="mt-0.5 flex items-center gap-3">
                    <span className="font-mono text-3xl font-bold tracking-[0.2em] text-ink">{code || '······'}</span>
                    <button
                        onClick={() => void copy()}
                        title="Copiar código"
                        className="rounded-lg p-1.5 text-mut transition-colors hover:bg-hover hover:text-ink"
                    >
                        <Copy size={16}/>
                    </button>
                </div>
            </div>
        </div>
    )
}
