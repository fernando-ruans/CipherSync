import {useEffect, useRef, useState} from 'react'
import toast from 'react-hot-toast'
import {
    AlertTriangle,
    Copy,
    Dices,
    Globe,
    History,
    KeyRound,
    Lock,
    Pencil,
    Save,
    ShieldCheck,
    Star,
    Trash2,
    User,
} from 'lucide-react'
import {useApp, setUnsavedItem} from '../state'
import {useT} from '../lib/locales'
import {api, errorMessage} from '../lib/api'
import {Button, IconButton, Input, RevealInput} from './ui'
import {GeneratorModal} from './GeneratorModal'
import {VersionHistoryModal} from './VersionHistory'
import {TagInput} from './TagInput'
import {TOTPSetupModal} from './TOTPSetupModal'
import {TOTPDisplay} from './TOTPDisplay'
import {PasskeyModal} from './PasskeyModal'
import type {PasskeyData} from '../lib/types'
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
    const t = useT()
    return (
        <div className="flex items-end gap-2">
            <div className="flex-1">
                <RevealInput label={label} value={value} onChange={onChange} placeholder={placeholder}/>
            </div>
            <IconButton title={t('common.copy')} onClick={() => void safeCopy(value)}>
                <Copy size={15}/>
            </IconButton>
            {onGenerate && (
                <IconButton title={t('common.generate')} onClick={onGenerate}>
                    <Dices size={15}/>
                </IconButton>
            )}
        </div>
    )
}

function PasskeySection({data, onEdit, onRemove}: {
    data: PasskeyData
    onEdit: () => void
    onRemove?: () => void
}) {
    const t = useT()
    const refMode = !data.privateKey.trim()
    return (
        <div className="rounded-xl border border-edge bg-input p-4">
            <div className="mb-3 flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-wider text-mut">{t('passkey.badge')}</span>
                <div className="flex gap-1">
                    <IconButton title={t('passkey.edit')} onClick={onEdit}>
                        <Pencil size={14}/>
                    </IconButton>
                    {onRemove && (
                        <IconButton title={t('passkey.remove')} onClick={onRemove} className="text-red-400 hover:bg-red-500/10">
                            <Trash2 size={14}/>
                        </IconButton>
                    )}
                </div>
            </div>
            {refMode && (
                <p className="mb-3 rounded-lg border border-amber-400/30 bg-amber-400/10 px-3 py-2 text-xs text-amber-400">
                    {t('passkey.refOnly')}
                </p>
            )}
            <div className="space-y-2 text-sm">
                <PasskeyRow label="RP ID" value={data.rpId}/>
                {data.rpName && <PasskeyRow label={t('passkey.rpName')} value={data.rpName}/>}
                {data.username && <PasskeyRow label={t('passkey.username')} value={data.username}/>}
                <PasskeyRow label={t('passkey.credentialIdLabel')} value={data.credentialId} mono truncate/>
                {!refMode && <PasskeyRow label={t('passkey.privateKeyLabel')} value="••••••••" mono secret={data.privateKey}/>}
                {data.transports.length > 0 && (
                    <div className="flex gap-1.5 pt-1">
                        {data.transports.map((t) => (
                            <span key={t} className="rounded-md bg-white/5 px-2 py-0.5 text-xs text-mut">{t}</span>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}

function PasskeyRow({label, value, mono, truncate, secret}: {
    label: string
    value: string
    mono?: boolean
    truncate?: boolean
    secret?: string
}) {
    const display = secret ?? value
    const t = useT()
    return (
        <div className="flex items-center justify-between gap-2">
            <span className="shrink-0 text-xs text-faint">{label}</span>
            <span className="flex min-w-0 items-center gap-1.5">
                <span className={`truncate text-soft ${mono ? 'font-mono text-[13px]' : ''}`} title={truncate ? display : undefined}>
                    {truncate && display.length > 24 ? `${display.slice(0, 12)}…${display.slice(-8)}` : display}
                </span>
                <button
                    type="button"
                    title={t('common.copy')}
                    onClick={() => void safeCopy(secret ?? value)}
                    className="shrink-0 rounded-md p-1 text-mut hover:bg-hover hover:text-ink"
                >
                    <Copy size={13}/>
                </button>
            </span>
        </div>
    )
}

function CopyRow({label, value, placeholder, onChange}: {
    label: string
    value: string
    onChange: (v: string) => void
    placeholder?: string
}) {
    const t = useT()
    return (
        <div className="flex items-end gap-2">
            <div className="flex-1">
                <Input label={label} value={value} onChange={onChange} placeholder={placeholder}/>
            </div>
            <IconButton title={t('common.copy')} onClick={() => void safeCopy(value)}>
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
    const lang = useApp((s) => s.lang)
    const t = useT()

    const item = items.find((i) => i.id === selectedId) ?? null

    const [draft, setDraft] = useState<Item | null>(null)
    const [saving, setSaving] = useState(false)
    const [showGenerator, setShowGenerator] = useState<'password' | 'field' | null>(null)
    const [generatorField, setGeneratorField] = useState('')
    const [showHistory, setShowHistory] = useState(false)
    const [showTOTPSetup, setShowTOTPSetup] = useState(false)
    const [showPasskeyModal, setShowPasskeyModal] = useState(false)
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
                <p className="mt-4 text-sm text-faint">{t('detail.selectItem')}</p>
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
        if (hasFields && !confirm(t('detail.changeTypeConfirm'))) {
            return
        }
        setDraft({...draft, type, fields: {}})
    }

    async function save() {
        if (!draft) return
        setSaving(true)
        try {
            await updateItem(draft)
            toast.success(t('detail.saved'))
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setSaving(false)
        }
    }
    saveRef.current = save

    async function del() {
        if (!item) return
        if (!confirm(t('detail.moveTrashConfirm', {title: item.title}))) return
        try {
            await removeItem(item.id)
            toast.success(t('detail.movedTrash'))
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
            toast.success(t('detail.2faSaved'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function savePasskey(data: PasskeyData) {
        if (!draft) return
        try {
            await updateItem({...draft, passkey: data})
            setDraft({...draft, passkey: data})
            setShowPasskeyModal(false)
            toast.success(t('passkey.saved'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function removeTOTP() {
        if (!draft) return
        if (!confirm(t('detail.remove2faConfirm'))) return
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
                            {ITEM_TYPES.map((it) => (
                                <option key={it.value} value={it.value}>
                                    {t(`type.${it.value}`)}
                                </option>
                            ))}
                        </select>
                    </div>
                    <input
                        value={draft.title}
                        onChange={(e) => set({title: e.target.value})}
                        placeholder={t('detail.titlePh')}
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
                            <AlertTriangle size={13}/> {t('detail.breached')}
                        </div>
                    )}
                </div>
                <div className="flex shrink-0 items-center gap-1">
                    <IconButton
                        title={draft.favorite ? t('detail.favRemove') : t('detail.favAdd')}
                        onClick={() => set({favorite: !draft.favorite})}
                        className={draft.favorite ? 'text-amber-400' : ''}
                    >
                        <Star size={18} fill={draft.favorite ? 'currentColor' : 'none'}/>
                    </IconButton>
                    <IconButton title={t('detail.history')} onClick={() => setShowHistory(true)}>
                        <History size={18}/>
                    </IconButton>
                    <IconButton title={t('detail.delete')} onClick={() => void del()} className="text-red-400 hover:bg-red-500/10">
                        <Trash2 size={18}/>
                    </IconButton>
                </div>
            </div>

            <div className="space-y-4">
                {draft.type === 'login' && (
                    <>
                        <CopyRow label={t('detail.username')} value={draft.username} onChange={(v) => set({username: v})} placeholder={t('detail.usernamePh')}/>
                        <SecretRow label={t('detail.password')} value={draft.password} onChange={(v) => set({password: v})} onGenerate={() => openGenerator('password')}/>
                        <div className="flex items-end gap-2">
                            <div className="flex-1">
                                <Input label={t('detail.siteUrl')} value={draft.url} onChange={(v) => set({url: v})} placeholder={t('detail.siteUrlPh')}/>
                            </div>
                        </div>

                        {item.totpSecret ? (
                            <div>
                                <TOTPDisplay itemId={item.id}/>
                                <button
                                    onClick={() => void removeTOTP()}
                                    className="mt-2 text-xs text-faint transition-colors hover:text-red-400"
                                >
                                    {t('detail.remove2fa')}
                                </button>
                            </div>
                        ) : (
                            <Button variant="subtle" onClick={() => setShowTOTPSetup(true)}>
                                <ShieldCheck size={16}/> {t('detail.add2fa')}
                            </Button>
                        )}

                        {draft.passkey ? (
                            <PasskeySection
                                data={draft.passkey}
                                onEdit={() => setShowPasskeyModal(true)}
                                onRemove={() => set({passkey: undefined})}
                            />
                        ) : (
                            <Button variant="subtle" onClick={() => setShowPasskeyModal(true)}>
                                <KeyRound size={16}/> {t('detail.attachPasskey')}
                            </Button>
                        )}
                    </>
                )}

                {draft.type === 'passkey' && (
                    draft.passkey ? (
                        <PasskeySection
                            data={draft.passkey}
                            onEdit={() => setShowPasskeyModal(true)}
                            onRemove={undefined}
                        />
                    ) : (
                        <Button variant="subtle" onClick={() => setShowPasskeyModal(true)}>
                            <KeyRound size={16}/> {t('detail.setupPasskey')}
                        </Button>
                    )
                )}

                {(draft.type === 'credit_card' || draft.type === 'identity') && (
                    <div className="grid grid-cols-2 gap-3">
                        {(TYPE_FIELDS[draft.type as keyof typeof TYPE_FIELDS] ?? []).map((f) =>
                            f.secret ? (
                                <div key={f.key} className="col-span-2">
                                    <SecretRow
                                        label={t(`field.${f.key}`)}
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
                                        label={t(`field.${f.key}`)}
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
                    <Input label={t('detail.category')} value={draft.category} onChange={(v) => set({category: v})} placeholder={t('detail.categoryPh')}/>
                </div>

                <TagInput tags={draft.tags ?? []} onChange={(tags) => set({tags})} suggestions={tagSuggestions}/>

                <div>
                    <span className="mb-1.5 block text-xs font-medium text-mut">{t('detail.notes')}</span>
                    <textarea
                        value={draft.notes}
                        onChange={(e) => set({notes: e.target.value})}
                        rows={4}
                        placeholder={t('detail.notesPh')}
                        className="w-full resize-none rounded-lg border border-edge bg-input px-3 py-2 text-sm text-ink placeholder:text-faint outline-none transition-colors focus:border-indigo-500/60"
                    />
                </div>
            </div>

            <AttachmentsSection itemId={item.id}/>

            <div className="mt-6 flex items-center justify-between border-t border-edge pt-4">
                <div className="flex items-center gap-4 text-xs text-faint">
                    <span className="flex items-center gap-1">
                        <User size={12}/> {draft.username || t('detail.noUser')}
                    </span>
                    <span>{t('detail.createdAt')} {new Date(draft.createdAt).toLocaleDateString(lang === 'en' ? 'en-US' : 'pt-BR')}</span>
                </div>
                <Button onClick={() => void save()} disabled={!dirty || saving} className="px-6">
                    <Save size={16}/> {saving ? t('common.saving') : t('common.save')}
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
            {showPasskeyModal && (
                <PasskeyModal
                    initial={draft.passkey ?? null}
                    onClose={() => setShowPasskeyModal(false)}
                    onSave={(data) => void savePasskey(data)}
                />
            )}
        </div>
    )
}
