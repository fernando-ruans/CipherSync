import {useEffect, useRef, useState} from 'react'
import toast from 'react-hot-toast'
import {
    AlertTriangle,
    Copy,
    Dices,
    Globe,
    History,
    Lock,
    Save,
    ShieldCheck,
    Star,
    Trash2,
    User,
} from 'lucide-react'
import {useApp, setUnsavedItem} from '../state'
import {api, errorMessage} from '../lib/api'
import {Button, IconButton, Input, RevealInput} from './ui'
import {GeneratorModal} from './GeneratorModal'
import {VersionHistoryModal} from './VersionHistory'
import {TagInput} from './TagInput'
import {TOTPSetupModal} from './TOTPSetupModal'
import {TOTPDisplay} from './TOTPDisplay'
import {AttachmentsSection} from './AttachmentsSection'
import {TYPE_FIELDS, ITEM_TYPES} from '../lib/fields'
import {extractDomain, safeCopy} from '../lib/util'
import type {Item, ItemType} from '../lib/types'

function SecretRow({
    label,
    value,
    onChange,
    placeholder,
    onGenerate,
}: {
    label: string
    value: string
    onChange: (v: string) => void
    placeholder?: string
    onGenerate?: () => void
}) {
    return (
        <div className="flex items-end gap-2">
            <div className="flex-1">
                <RevealInput label={label} value={value} onChange={onChange} placeholder={placeholder}/>
            </div>
            <IconButton title="Copiar" onClick={() => void safeCopy(value)}>
                <Copy size={15}/>
            </IconButton>
            {onGenerate && (
                <IconButton title="Gerar" onClick={onGenerate}>
                    <Dices size={15}/>
                </IconButton>
            )}
        </div>
    )
}

function CopyRow({label, value, placeholder, onChange}: {
    label: string
    value: string
    onChange: (v: string) => void
    placeholder?: string
}) {
    return (
        <div className="flex items-end gap-2">
            <div className="flex-1">
                <Input label={label} value={value} onChange={onChange} placeholder={placeholder}/>
            </div>
            <IconButton title="Copiar" onClick={() => void safeCopy(value)}>
                <Copy size={15}/>
            </IconButton>
        </div>
    )
}

export function ItemDetail() {
    const items = useApp((s) => s.items)
    const selectedId = useApp((s) => s.selectedId)
    const updateItem = useApp((s) => s.updateItem)
    const removeItem = useApp((s) => s.removeItem)
    const breached = useApp((s) => s.breachedIds.includes(selectedId ?? ''))

    const item = items.find((i) => i.id === selectedId) ?? null

    const [draft, setDraft] = useState<Item | null>(null)
    const [saving, setSaving] = useState(false)
    const [showGenerator, setShowGenerator] = useState<'password' | 'field' | null>(null)
    const [generatorField, setGeneratorField] = useState('')
    const [showHistory, setShowHistory] = useState(false)
    const [showTOTPSetup, setShowTOTPSetup] = useState(false)
    const appliedRef = useRef('')

    useEffect(() => {
        if (!item) {
            setDraft(null)
            appliedRef.current = ''
            setUnsavedItem(null)
            return
        }
        const key = `${item.id}:${item.updatedAt}`
        if (key === appliedRef.current) return // same content, keep the draft
        appliedRef.current = key
        setDraft({...item})
    }, [item])

    // register unsaved edits so navigation can ask for confirmation
    useEffect(() => {
        if (!item || !draft) {
            setUnsavedItem(null)
            return
        }
        setUnsavedItem(JSON.stringify(draft) !== JSON.stringify(item) ? item.id : null)
    }, [draft, item])

    // Ctrl+S saves the current item
    const saveRef = useRef<() => Promise<void>>(async () => {})
    useEffect(() => {
        function onKey(e: KeyboardEvent) {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
                e.preventDefault()
                void saveRef.current()
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [])

    if (!item || !draft) {
        return (
            <div className="flex h-full flex-col items-center justify-center text-center">
                <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-input">
                    <Lock size={26} className="text-faint"/>
                </div>
                <p className="mt-4 text-sm text-faint">Selecione um item ou crie um novo</p>
            </div>
        )
    }

    const dirty = JSON.stringify(draft) !== JSON.stringify(item)

    function set(patch: Partial<Item>) {
        if (!draft) return
        setDraft({...draft, ...patch})
    }

    function setField(key: string, value: string) {
        if (!draft) return
        setDraft({...draft, fields: {...draft.fields, [key]: value}})
    }

    function changeType(type: ItemType) {
        if (!draft) return
        const hasFields = Object.keys(draft.fields ?? {}).some((k) => (draft.fields[k] ?? '').trim() !== '')
        if (hasFields && !confirm('Trocar o tipo apaga os campos específicos preenchidos. Continuar?')) {
            return
        }
        setDraft({...draft, type, fields: {}})
    }

    async function save() {
        if (!draft) return
        setSaving(true)
        try {
            await updateItem(draft)
            toast.success('Alterações salvas')
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setSaving(false)
        }
    }
    saveRef.current = save

    async function del() {
        if (!item) return
        if (!confirm(`Mover "${item.title}" para a lixeira? Você pode restaurá-lo depois.`)) return
        try {
            await removeItem(item.id)
            toast.success('Item movido para a lixeira')
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    function openGenerator(fieldKey = 'password') {
        setGeneratorField(fieldKey)
        setShowGenerator('field')
    }

    function useGenerated(value: string) {
        if (generatorField === 'password') set({password: value})
        else setField(generatorField, value)
        setShowGenerator(null)
    }

    async function saveTOTP(secret: string) {
        if (!draft) return
        try {
            await updateItem({...draft, totpSecret: secret})
            setDraft({...draft, totpSecret: secret})
            setShowTOTPSetup(false)
            toast.success('2FA configurado')
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function removeTOTP() {
        if (!draft) return
        if (!confirm('Remover o 2FA deste item?')) return
        try {
            await updateItem({...draft, totpSecret: ''})
            setDraft({...draft, totpSecret: ''})
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    const domain = draft.url ? extractDomain(draft.url) : ''
    const tagSuggestions = [...new Set(items.flatMap((i) => i.tags ?? []))]

    return (
        <div className="flex h-full flex-col overflow-y-auto p-6">
            <div className="mb-4 flex items-start justify-between gap-4">
                <div className="min-w-0 flex-1">
                    <div className="mb-2 flex items-center gap-2">
                        <select
                            value={draft.type}
                            onChange={(e) => changeType(e.target.value as ItemType)}
                            className="rounded-lg border border-edge bg-input px-2 py-1 text-xs font-medium text-mut outline-none"
                        >
                            {ITEM_TYPES.map((t) => (
                                <option key={t.value} value={t.value}>
                                    {t.label}
                                </option>
                            ))}
                        </select>
                    </div>
                    <input
                        value={draft.title}
                        onChange={(e) => set({title: e.target.value})}
                        placeholder="Título"
                        className="w-full bg-transparent text-xl font-semibold text-ink outline-none placeholder:text-faint"
                    />
                    {domain && (
                        <a
                            href={draft.url}
                            target="_blank"
                            rel="noreferrer"
                            className="mt-1 flex items-center gap-1.5 text-sm text-accent hover:underline"
                        >
                            <Globe size={14}/> {domain}
                        </a>
                    )}
                    {breached && (
                        <div className="mt-2 flex w-fit items-center gap-1.5 rounded-lg border border-red-500/30 bg-red-500/10 px-2.5 py-1 text-xs font-medium text-red-400">
                            <AlertTriangle size={13}/> Senha encontrada em vazamentos — altere-a
                        </div>
                    )}
                </div>
                <div className="flex shrink-0 items-center gap-1">
                    <IconButton
                        title={draft.favorite ? 'Remover dos favoritos' : 'Marcar como favorito'}
                        onClick={() => set({favorite: !draft.favorite})}
                        className={draft.favorite ? 'text-amber-400' : ''}
                    >
                        <Star size={18} fill={draft.favorite ? 'currentColor' : 'none'}/>
                    </IconButton>
                    <IconButton title="Histórico de versões" onClick={() => setShowHistory(true)}>
                        <History size={18}/>
                    </IconButton>
                    <IconButton title="Excluir" onClick={() => void del()} className="text-red-400 hover:bg-red-500/10">
                        <Trash2 size={18}/>
                    </IconButton>
                </div>
            </div>

            <div className="space-y-4">
                {draft.type === 'login' && (
                    <>
                        <CopyRow label="Nome de usuário" value={draft.username} onChange={(v) => set({username: v})}/>
                        <SecretRow label="Senha" value={draft.password} onChange={(v) => set({password: v})} onGenerate={() => openGenerator('password')}/>
                        <div className="flex items-end gap-2">
                            <div className="flex-1">
                                <Input label="Site (URL)" value={draft.url} onChange={(v) => set({url: v})} placeholder="https://exemplo.com"/>
                            </div>
                        </div>

                        {item.totpSecret ? (
                            <div>
                                <TOTPDisplay itemId={item.id}/>
                                <button
                                    onClick={() => void removeTOTP()}
                                    className="mt-2 text-xs text-faint transition-colors hover:text-red-400"
                                >
                                    Remover 2FA
                                </button>
                            </div>
                        ) : (
                            <Button variant="subtle" onClick={() => setShowTOTPSetup(true)}>
                                <ShieldCheck size={16}/> Adicionar 2FA
                            </Button>
                        )}
                    </>
                )}

                {(draft.type === 'credit_card' || draft.type === 'identity') && (
                    <div className="grid grid-cols-2 gap-3">
                        {(TYPE_FIELDS[draft.type as keyof typeof TYPE_FIELDS] ?? []).map((f) =>
                            f.secret ? (
                                <div key={f.key} className="col-span-2">
                                    <SecretRow
                                        label={f.label}
                                        value={draft.fields[f.key] ?? ''}
                                        onChange={(v) => setField(f.key, v)}
                                        onGenerate={
                                            f.key === 'cvv' || f.key === 'pin'
                                                ? () => openGenerator(f.key)
                                                : undefined
                                        }
                                    />
                                </div>
                            ) : (
                                <div key={f.key} className={f.key === 'number' || f.key === 'cardholder' ? 'col-span-2' : ''}>
                                    <CopyRow
                                        label={f.label}
                                        value={draft.fields[f.key] ?? ''}
                                        onChange={(v) => setField(f.key, v)}
                                        placeholder={f.placeholder}
                                    />
                                </div>
                            ),
                        )}
                    </div>
                )}

                <div>
                    <Input label="Categoria" value={draft.category} onChange={(v) => set({category: v})} placeholder="e.g. Pessoal, Trabalho, Banco"/>
                </div>

                <TagInput tags={draft.tags ?? []} onChange={(tags) => set({tags})} suggestions={tagSuggestions}/>

                <div>
                    <span className="mb-1.5 block text-xs font-medium text-mut">Notas</span>
                    <textarea
                        value={draft.notes}
                        onChange={(e) => set({notes: e.target.value})}
                        rows={4}
                        placeholder="Anotações, respostas de segurança, etc."
                        className="w-full resize-none rounded-lg border border-edge bg-input px-3 py-2 text-sm text-ink placeholder:text-faint outline-none transition-colors focus:border-indigo-500/60"
                    />
                </div>
            </div>

            <AttachmentsSection itemId={item.id}/>

            <div className="mt-6 flex items-center justify-between border-t border-edge pt-4">
                <div className="flex items-center gap-4 text-xs text-faint">
                    <span className="flex items-center gap-1">
                        <User size={12}/> {draft.username || 'sem usuário'}
                    </span>
                    <span>Criado em {new Date(draft.createdAt).toLocaleDateString('pt-BR')}</span>
                </div>
                <Button onClick={() => void save()} disabled={!dirty || saving} className="px-6">
                    <Save size={16}/> {saving ? 'Salvando...' : 'Salvar'}
                </Button>
            </div>

            {showGenerator && (
                <GeneratorModal
                    onClose={() => setShowGenerator(null)}
                    onUse={useGenerated}
                    preset={generatorField === 'password' ? 'password' : 'pin'}
                />
            )}
            {showHistory && <VersionHistoryModal itemId={item.id} onClose={() => setShowHistory(false)}/>}
            {showTOTPSetup && <TOTPSetupModal onClose={() => setShowTOTPSetup(false)} onSave={saveTOTP}/>}
        </div>
    )
}
