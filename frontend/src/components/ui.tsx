import {ReactNode} from 'react'
import {Eye, EyeOff, X} from 'lucide-react'
import {useEffect, useRef, useState} from 'react'
import {scoreColor, passwordScore} from '../lib/password'
import {useT} from '../lib/locales'

// Stack of mounted modal closers (topmost last). Only the topmost modal
// handles Escape, so stacked modals close one at a time.
const modalStack: Array<() => void> = []

type Variant = 'primary' | 'ghost' | 'danger' | 'subtle'

const variantClasses: Record<Variant, string> = {
    primary:
        'bg-indigo-500 hover:bg-indigo-400 text-white shadow-lg shadow-indigo-500/25 disabled:opacity-50',
    ghost: 'text-mut hover:bg-hover hover:text-ink',
    danger: 'text-red-400 hover:bg-red-500/10',
    subtle: 'bg-input hover:bg-hover text-soft border border-edge',
}

export function Button({
    children,
    onClick,
    variant = 'primary',
    className = '',
    disabled = false,
    type = 'button',
    title,
}: {
    children: ReactNode
    onClick?: () => void
    variant?: Variant
    className?: string
    disabled?: boolean
    type?: 'button' | 'submit'
    title?: string
}) {
    return (
        <button
            type={type}
            title={title}
            disabled={disabled}
            onClick={onClick}
            className={`inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed ${variantClasses[variant]} ${className}`}
        >
            {children}
        </button>
    )
}

export function IconButton({
    children,
    onClick,
    className = '',
    title,
}: {
    children: ReactNode
    onClick?: () => void
    className?: string
    title?: string
}) {
    return (
        <button
            type="button"
            title={title}
            onClick={onClick}
            className={`inline-flex h-8 w-8 items-center justify-center rounded-lg text-mut transition-colors hover:bg-hover hover:text-ink ${className}`}
        >
            {children}
        </button>
    )
}

export function Input({
    label,
    value,
    onChange,
    placeholder,
    type = 'text',
    autoFocus,
    onEnter,
}: {
    label?: string
    value: string
    onChange: (v: string) => void
    placeholder?: string
    type?: string
    autoFocus?: boolean
    onEnter?: () => void
}) {
    return (
        <label className="block">
            {label && <span className="mb-1.5 block text-xs font-medium text-mut">{label}</span>}
            <input
                type={type}
                value={value}
                autoFocus={autoFocus}
                placeholder={placeholder}
                onChange={(e) => onChange(e.target.value)}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' && onEnter) onEnter()
                }}
                className="w-full rounded-lg border border-edge bg-input px-3 py-2 text-sm text-ink placeholder:text-faint outline-none transition-colors focus:border-indigo-500/60 focus:bg-hover"
            />
        </label>
    )
}

export function RevealInput({
    label,
    value,
    onChange,
    placeholder,
}: {
    label?: string
    value: string
    onChange: (v: string) => void
    placeholder?: string
}) {
    const t = useT()
    const [revealed, setRevealed] = useState(false)
    return (
        <div>
            {label && <span className="mb-1.5 block text-xs font-medium text-mut">{label}</span>}
            <div className="flex items-stretch gap-2">
                <input
                    type={revealed ? 'text' : 'password'}
                    value={value}
                    placeholder={placeholder}
                    onChange={(e) => onChange(e.target.value)}
                    className="w-full rounded-lg border border-edge bg-input px-3 py-2 text-sm text-ink placeholder:text-faint outline-none transition-colors focus:border-indigo-500/60 focus:bg-hover"
                />
                <button
                    type="button"
                    onClick={() => setRevealed(!revealed)}
                    title={revealed ? t('common.hide') : t('common.show')}
                    className="flex h-auto w-10 shrink-0 items-center justify-center rounded-lg border border-edge bg-input text-mut hover:text-ink"
                >
                    {revealed ? <EyeOff size={16}/> : <Eye size={16}/>}
                </button>
            </div>
        </div>
    )
}

export function StrengthMeter({password}: {password: string}) {
    const t = useT()
    const score = passwordScore(password)
    if (!password) return null
    return (
        <div className="mt-2">
            <div className="flex h-1.5 w-full gap-1">
                {[0, 1, 2, 3].map((i) => (
                    <div
                        key={i}
                        className="h-full flex-1 rounded-full transition-colors"
                        style={{
                            backgroundColor: i <= score ? scoreColor(score) : 'var(--edge)',
                        }}
                    />
                ))}
            </div>
            <div className="mt-1 text-xs" style={{color: scoreColor(score)}}>
                {t(`strength.${score}`)}
            </div>
        </div>
    )
}

export function Modal({
    title,
    onClose,
    children,
    width = 'max-w-lg',
}: {
    title: string
    onClose: () => void
    children: ReactNode
    width?: string
}) {
    const closeRef = useRef<() => void>(onClose)
    closeRef.current = onClose
    useEffect(() => {
        // topmost-stack: only the last mounted modal responds to Escape
        const entry = () => closeRef.current()
        modalStack.push(entry)
        function onKey(e: KeyboardEvent) {
            if (e.key === 'Escape' && modalStack[modalStack.length - 1] === entry) {
                e.stopPropagation()
                entry()
            }
        }
        window.addEventListener('keydown', onKey, true)
        return () => {
            const i = modalStack.indexOf(entry)
            if (i >= 0) modalStack.splice(i, 1)
            window.removeEventListener('keydown', onKey, true)
        }
    }, [])
    const t = useT()
    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-6 backdrop-blur-sm">
            <div className={`w-full ${width} rounded-2xl border border-edge bg-surface shadow-2xl`}>
                <div className="flex items-center justify-between border-b border-edge px-5 py-4">
                    <h2 className="text-sm font-semibold text-ink">{title}</h2>
                    <IconButton onClick={onClose} title={t('common.close')}>
                        <X size={16}/>
                    </IconButton>
                </div>
                <div className="max-h-[70vh] overflow-y-auto p-5">{children}</div>
            </div>
        </div>
    )
}
