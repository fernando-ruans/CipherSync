import {useEffect, useMemo, useRef, useState} from 'react'
import {Search, X} from 'lucide-react'
import {useApp} from '../state'
import {api} from '../lib/api'
import {safeCopy} from '../lib/util'
import {useT} from '../lib/locales'
import {EventsOn} from '../../wailsjs/runtime/runtime'

export function QuickAccess() {
    const t = useT()
    const [open, setOpen] = useState(false)
    const [query, setQuery] = useState('')
    const [index, setIndex] = useState(0)
    const items = useApp((s) => s.items)
    const inputRef = useRef<HTMLInputElement>(null)

    useEffect(() => {
        const off = EventsOn('quick-access-open', () => {
            setQuery('')
            setIndex(0)
            setOpen(true)
        })
        const offClose = EventsOn('quick-access-close', () => {
            setOpen(false)
        })
        return () => {
            off()
            offClose()
        }
    }, [])

    useEffect(() => {
        if (!open) return
        const id = setTimeout(() => inputRef.current?.focus(), 50)
        return () => clearTimeout(id)
    }, [open])

    const results = useMemo(() => {
        const q = query.trim().toLowerCase()
        const pool = items.filter((i) => !i.deleted && (i.type === 'login' || i.type === 'passkey') && (i.password || i.passkey))
        if (q === '') return pool.slice(0, 8)
        return pool
            .filter(
                (i) =>
                    i.title.toLowerCase().includes(q) ||
                    i.username.toLowerCase().includes(q) ||
                    i.url.toLowerCase().includes(q),
            )
            .slice(0, 8)
    }, [items, query])

    useEffect(() => {
        setIndex(0)
    }, [query])

    async function close() {
        setOpen(false)
        try {
            await api.closeQuickAccess()
        } catch {
            // ignore
        }
    }

    async function choose(id: string) {
        const item = items.find((i) => i.id === id)
        if (!item) return
        // passkey-only items have no password: fall back to the username
        if (item.password) {
            await safeCopy(item.password, t('qa.copied'))
        } else if (item.username) {
            await safeCopy(item.username, t('common.copied'))
        } else {
            await close()
            return
        }
        await close()
    }

    function onKey(e: React.KeyboardEvent) {
        if (e.key === 'Escape') {
            e.preventDefault()
            void close()
        } else if (e.key === 'ArrowDown') {
            e.preventDefault()
            setIndex((i) => Math.min(i + 1, results.length - 1))
        } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            setIndex((i) => Math.max(i - 1, 0))
        } else if (e.key === 'Enter') {
            e.preventDefault()
            const sel = results[index]
            if (sel) void choose(sel.id)
        }
    }

    if (!open) return null

    return (
        <div className="fixed inset-0 z-[100] flex items-start justify-center bg-black/50 pt-[12vh] backdrop-blur-sm">
            <div className="w-full max-w-lg overflow-hidden rounded-2xl border border-edge bg-surface shadow-2xl">
                <div className="flex items-center gap-2 border-b border-edge px-4 py-3">
                    <Search size={16} className="shrink-0 text-faint"/>
                    <input
                        ref={inputRef}
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        onKeyDown={onKey}
                        placeholder={t('qa.placeholder')}
                        className="w-full bg-transparent text-sm text-ink placeholder:text-faint outline-none"
                    />
                    <button
                        onClick={() => void close()}
                        className="rounded-lg p-1 text-mut hover:bg-hover hover:text-ink"
                        title={t('common.close')}
                    >
                        <X size={16}/>
                    </button>
                </div>
                <div className="max-h-72 overflow-y-auto p-2">
                    {results.length === 0 ? (
                        <div className="px-3 py-6 text-center text-sm text-faint">{t('qa.noResults')}</div>
                    ) : (
                        results.map((item, i) => (
                            <button
                                key={item.id}
                                onClick={() => void choose(item.id)}
                                onMouseEnter={() => setIndex(i)}
                                className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm transition-colors ${
                                    i === index ? 'bg-accent/15 text-accent' : 'text-soft hover:bg-hover'
                                }`}
                            >
                                <span className="truncate font-medium">{item.title || t('detail.noTitle')}</span>
                                <span className="ml-2 shrink-0 truncate text-xs text-faint">{item.username}</span>
                            </button>
                        ))
                    )}
                </div>
                <div className="border-t border-edge px-4 py-2 text-[11px] text-faint">
                    {t('qa.hint')}
                </div>
            </div>
        </div>
    )
}
