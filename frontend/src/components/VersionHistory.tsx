import {useEffect, useState} from 'react'
import toast from 'react-hot-toast'
import {History, RotateCcw} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {useApp} from '../state'
import {Button, Modal} from './ui'
import {useT} from '../lib/locales'
import type {Item, VersionEntry} from '../lib/types'
import {FIELD_LABELS} from '../lib/fields'

type TFn = (key: string) => string

function diffLabels(t: TFn, current: Item, old: Item): {label: string; old: string; current: string}[] {
    const out: {label: string; old: string; current: string}[] = []
    const simple: [keyof Item, string][] = [
        ['title', t('import.fieldTitle')],
        ['username', t('import.fieldUsername')],
        ['password', t('import.fieldPassword')],
        ['url', t('import.fieldUrl')],
        ['notes', t('import.fieldNotes')],
        ['category', t('import.fieldCategory')],
    ]
    for (const [key, label] of simple) {
        if ((current[key] ?? '') !== (old[key] ?? '')) {
            out.push({label, old: (old[key] ?? '') as string, current: (current[key] ?? '') as string})
        }
    }
    const allFieldKeys = new Set([
        ...Object.keys(current.fields ?? {}),
        ...Object.keys(old.fields ?? {}),
    ])
    for (const k of allFieldKeys) {
        const a = old.fields?.[k] ?? ''
        const b = current.fields?.[k] ?? ''
        if (a !== b) {
            const fk = `field.${k}` as Parameters<TFn>[0]
            out.push({label: t(fk) !== fk ? t(fk) : (FIELD_LABELS[k] ?? k), old: a, current: b})
        }
    }
    const currentTags = (current.tags ?? []).join(', ')
    const oldTags = (old.tags ?? []).join(', ')
    if (currentTags !== oldTags) {
        out.push({label: t('main.tags'), old: oldTags, current: currentTags})
    }
    return out
}

export function VersionHistoryModal({itemId, onClose}: {itemId: string; onClose: () => void}) {
    const t = useT()
    const lang = useApp((s) => s.lang)
    const [versions, setVersions] = useState<VersionEntry[] | null>(null)
    const [selected, setSelected] = useState<VersionEntry | null>(null)
    const [restoring, setRestoring] = useState(false)
    const current = useApp((s) => s.items.find((i) => i.id === itemId))
    const restoreVersion = useApp((s) => s.restoreVersion)

    useEffect(() => {
        void (async () => {
            try {
                const v = await api.getItemVersions(itemId)
                setVersions(v)
                if (v.length > 0) setSelected(v[0])
            } catch (err) {
                toast.error(await errorMessage(err))
                onClose()
            }
        })()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [itemId])

    async function restore() {
        if (!selected) return
        setRestoring(true)
        try {
            await restoreVersion(selected.id)
            toast.success(t('history.restored'))
            onClose()
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setRestoring(false)
        }
    }

    return (
        <Modal title={t('history.title')} onClose={onClose} width="max-w-3xl">
            {!versions ? (
                <p className="py-8 text-center text-sm text-mut">{t('history.loading')}</p>
            ) : versions.length === 0 ? (
                <p className="flex flex-col items-center gap-3 py-8 text-center text-sm text-mut">
                    <History size={28} className="text-faint"/>
                    {t('history.empty')}
                </p>
            ) : (
                <div className="flex gap-5">
                    <div className="w-52 shrink-0 space-y-1">
                        {versions.map((v) => (
                            <button
                                key={v.id}
                                onClick={() => setSelected(v)}
                                className={`w-full rounded-lg px-3 py-2 text-left text-xs transition-colors ${
                                    selected?.id === v.id ? 'bg-accent/15 text-accent' : 'text-mut hover:bg-hover'
                                }`}
                            >
                                <div className="font-medium">{new Date(v.timestamp).toLocaleString(lang === 'en' ? 'en-US' : 'pt-BR')}</div>
                                <div className="mt-0.5 text-[11px] opacity-70">v{v.timestamp}</div>
                            </button>
                        ))}
                    </div>

                    <div className="min-w-0 flex-1">
                        {selected && current ? (
                            <div className="space-y-2">
                                {diffLabels(t, current, selected.item).length === 0 && (
                                    <p className="text-sm text-mut">{t('history.noDiff')}</p>
                                )}
                                {diffLabels(t, current, selected.item).map((d, i) => (
                                    <div key={i} className="rounded-lg border border-edge bg-input p-3">
                                        <div className="mb-1 text-xs font-semibold text-mut">{d.label}</div>
                                        <div className="text-sm">
                                            <span className="text-red-400 line-through decoration-red-400/60">
                                                {d.old || '—'}
                                            </span>
                                            <span className="mx-2 text-faint">→</span>
                                            <span className="text-emerald-400">{d.current || '—'}</span>
                                        </div>
                                    </div>
                                ))}
                                <div className="pt-3">
                                    <Button
                                        onClick={() => void restore()}
                                        disabled={restoring}
                                        variant="subtle"
                                        className="w-full"
                                    >
                                        <RotateCcw size={15}/> {t('history.restore')}
                                    </Button>
                                </div>
                            </div>
                        ) : (
                            <p className="text-sm text-mut">{t('history.select')}</p>
                        )}
                    </div>
                </div>
            )}
        </Modal>
    )
}
