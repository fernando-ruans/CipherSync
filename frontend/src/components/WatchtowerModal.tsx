import {useEffect, useState} from 'react'
import toast from 'react-hot-toast'
import {AlertTriangle, ArrowRight, CalendarX, KeyRound, Loader2, ShieldCheck, Shield} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {useApp} from '../state'
import {Modal} from './ui'
import type {HealthReport, ItemRef} from '../lib/types'

function scoreColor(score: number): string {
    if (score >= 80) return '#10b981'
    if (score >= 50) return '#f59e0b'
    return '#ef4444'
}

function StatCard({label, count, color, icon, onClick}: {
    label: string
    count: number
    color: string
    icon: React.ReactNode
    onClick?: () => void
}) {
    return (
        <button
            onClick={onClick}
            disabled={!onClick || count === 0}
            className="flex flex-col items-center gap-1 rounded-xl border border-edge bg-input p-3 transition-colors hover:bg-hover disabled:opacity-50"
        >
            <span style={{color}}>{icon}</span>
            <span className="text-xl font-bold" style={{color}}>{count}</span>
            <span className="text-[11px] text-mut">{label}</span>
        </button>
    )
}

function ItemRow({ref, onSelect}: {ref: ItemRef; onSelect: (id: string) => void}) {
    const color = ref.score >= 3 ? 'text-emerald-400' : ref.score === 2 ? 'text-amber-400' : 'text-red-400'
    return (
        <button
            onClick={() => onSelect(ref.id)}
            className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors hover:bg-hover"
        >
            <span className="truncate text-soft">{ref.title || 'Sem título'}</span>
            <span className="flex items-center gap-2">
                <span className={`text-xs font-medium ${color}`}>
                    {'•'.repeat(Math.max(1, ref.score))}
                </span>
                <ArrowRight size={14} className="text-faint"/>
            </span>
        </button>
    )
}

type Section = 'weak' | 'dup' | 'old' | '2fa' | 'breach'

export function WatchtowerModal({onClose, onSelectItem}: {
    onClose: () => void
    onSelectItem: (id: string) => void
}) {
    const setBreachedIds = useApp((s) => s.setBreachedIds)
    const [report, setReport] = useState<HealthReport | null>(null)
    const [section, setSection] = useState<Section>('weak')

    useEffect(() => {
        void (async () => {
            try {
                const r = await api.analyzeVault()
                setReport(r)
                setBreachedIds(r.breachedItems.map((i) => i.id))
            } catch (err) {
                toast.error(await errorMessage(err))
            }
        })()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    if (!report) {
        return (
            <Modal title="Watchtower" onClose={onClose}>
                <div className="flex items-center justify-center gap-2 py-10 text-mut">
                    <Loader2 size={18} className="animate-spin"/> Analisando senhas...
                </div>
            </Modal>
        )
    }

    function select(id: string) {
        onSelectItem(id)
        onClose()
    }

    const sections: {key: Section; label: string; count: number; items: ItemRef[]; empty: string}[] = [
        {key: 'weak', label: 'Fracas', count: report.weakCount, items: report.weakItems, empty: 'Nenhuma senha fraca '},
        {key: 'dup', label: 'Duplicadas', count: report.duplicateCount, items: [], empty: 'Nenhuma senha duplicada '},
        {key: 'old', label: 'Antigas', count: report.oldCount, items: report.oldItems, empty: 'Nenhuma senha antiga '},
        {key: '2fa', label: 'Sem 2FA', count: report.missing2FA, items: report.missing2FAItems, empty: 'Tudo com 2FA '},
        {key: 'breach', label: 'Vazadas', count: report.breachedCount, items: report.breachedItems, empty: 'Nenhuma senha vazada '},
    ]

    return (
        <Modal title="Watchtower" onClose={onClose} width="max-w-2xl">
            <div className="mb-5 rounded-xl border border-edge bg-input p-4">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 text-sm font-semibold text-ink">
                        <Shield size={18} className="text-accent"/> Nível de segurança
                    </div>
                    <span className="text-2xl font-bold" style={{color: scoreColor(report.totalScore)}}>
                        {report.totalScore}%
                    </span>
                </div>
                <div className="mt-3 h-2 w-full overflow-hidden rounded-full bg-white/5">
                    <div
                        className="h-full rounded-full transition-all"
                        style={{
                            width: `${report.totalScore}%`,
                            backgroundColor: scoreColor(report.totalScore),
                        }}
                    />
                </div>
                <p className="mt-2 text-xs text-faint">
                    {report.totalPasswords} senha(s) analisada(s) em {report.totalItems} item(ns).
                    {report.breachCheckError && ' (verificação de vazamento indisponível sem internet)'}
                </p>
            </div>

            <div className="mb-4 grid grid-cols-5 gap-2">
                {sections.map((s) => (
                    <StatCard
                        key={s.key}
                        label={s.label}
                        count={s.count}
                        color={s.key === 'breach' ? '#ef4444' : s.key === '2fa' ? '#f59e0b' : s.key === 'dup' ? '#a855f7' : '#64748b'}
                        icon={
                            s.key === 'breach' ? <AlertTriangle size={18}/> :
                            s.key === '2fa' ? <KeyRound size={18}/> :
                            s.key === 'old' ? <CalendarX size={18}/> : <ShieldCheck size={18}/>
                        }
                        onClick={() => setSection(s.key)}
                    />
                ))}
            </div>

            <div className="rounded-xl border border-edge bg-input p-3">
                <div className="flex items-center justify-between border-b border-edge px-1 pb-2">
                    <span className="text-sm font-semibold text-ink">
                        {sections.find((s) => s.key === section)?.label}
                    </span>
                    <span className="text-xs text-faint">{sections.find((s) => s.key === section)?.count}</span>
                </div>

                {section === 'dup' ? (
                    <div className="mt-2 max-h-64 space-y-2 overflow-y-auto">
                        {report.duplicateGroups.length === 0 ? (
                            <p className="px-2 py-6 text-center text-sm text-faint">Nenhuma senha duplicada </p>
                        ) : (
                            report.duplicateGroups.map((g, i) => (
                                <div key={i} className="rounded-lg border border-edge bg-panel2 p-2">
                                    <div className="mb-1 flex items-center justify-between px-1 text-xs">
                                        <span className="font-mono text-mut" title="Senha oculta por segurança">
                                            {g.password.slice(0, 2)}{'•'.repeat(Math.max(4, Math.min(10, g.password.length - 2)))}
                                        </span>
                                        <span className="text-faint">{g.items.length} itens</span>
                                    </div>
                                    {g.items.map((r) => (
                                        <ItemRow key={r.id} ref={r} onSelect={select}/>
                                    ))}
                                </div>
                            ))
                        )}
                    </div>
                ) : (
                    <div className="mt-2 max-h-64 space-y-0.5 overflow-y-auto">
                        {sections.find((s) => s.key === section)?.items.length === 0 ? (
                            <p className="px-2 py-6 text-center text-sm text-faint">
                                {sections.find((s) => s.key === section)?.empty}
                            </p>
                        ) : (
                            sections.find((s) => s.key === section)?.items.map((r) => (
                                <ItemRow key={r.id} ref={r} onSelect={select}/>
                            ))
                        )}
                    </div>
                )}
            </div>
        </Modal>
    )
}
