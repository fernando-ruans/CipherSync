import {useState} from 'react'
import toast from 'react-hot-toast'
import {
    ChevronLeft,
    KeyRound,
    Loader2,
    Plus,
    ShieldCheck,
    Trash2,
    Vault,
} from 'lucide-react'
import {useApp} from '../state'
import {api, errorMessage} from '../lib/api'
import {Button, Input, RevealInput, StrengthMeter} from './ui'
import {useT} from '../lib/locales'
import type {VaultInfo} from '../lib/types'
import logo from '../assets/ciphersync-logo-128.png'

function Shell({children}: {children: React.ReactNode}) {
    const t = useT()
    return (
        <div className="flex h-full items-center justify-center bg-[radial-gradient(ellipse_at_top,rgba(99,102,241,0.15),transparent_60%)]">
            <div className="w-full max-w-sm">
                <div className="mb-8 flex flex-col items-center text-center">
                    <img src={logo} alt="CipherSync" className="mb-4 h-20 w-20 rounded-2xl object-contain drop-shadow-lg"/>
                    <h1 className="text-2xl font-bold tracking-tight">
                        <span className="text-ink">Cipher</span>
                        <span style={{color: '#3142cb'}}>Sync</span>
                    </h1>
                    <p className="mt-1 text-sm text-mut">{t('auth.tagline')}</p>
                </div>
                <div className="rounded-2xl border border-edge bg-surface p-6 shadow-2xl">{children}</div>
            </div>
        </div>
    )
}

function LoadingButton({loading, children}: {loading: boolean; children: React.ReactNode}) {
    return (
        <Button type="submit" disabled={loading} className="w-full py-2.5">
            {loading ? <Loader2 size={16} className="animate-spin"/> : children}
        </Button>
    )
}

export function SetupScreen() {
    const t = useT()
    const setup = useApp((s) => s.setup)
    const [name, setName] = useState('')
    const [password, setPassword] = useState('')
    const [confirm, setConfirm] = useState('')
    const [loading, setLoading] = useState(false)

    async function submit() {
        setLoading(true)
        try {
            await setup(name, password, confirm)
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    return (
        <Shell>
            <h2 className="mb-1 text-lg font-semibold text-ink">{t('auth.setupTitle')}</h2>
            <p className="mb-5 text-sm text-mut">
                {t('auth.setupDesc')} <span className="text-soft">{t('auth.noRecover')}</span>
            </p>
            <form onSubmit={(e) => {
                e.preventDefault()
                void submit()
            }} className="space-y-4">
                <Input label={t('auth.vaultName')} value={name} onChange={setName} placeholder={t('auth.vaultNamePh')} autoFocus/>
                <div>
                    <RevealInput label={t('auth.masterPw')} value={password} onChange={setPassword}/>
                    <StrengthMeter password={password}/>
                </div>
                <Input
                    label={t('auth.confirmPw')}
                    type="password"
                    value={confirm}
                    onChange={setConfirm}
                    onEnter={() => void submit()}
                />
                {password.length < 8 && (
                    <p className="flex items-center gap-1.5 text-xs text-amber-400">
                        <ShieldCheck size={14}/> {t('auth.min8')}
                    </p>
                )}
                <LoadingButton loading={loading}>{t('auth.createVault')}</LoadingButton>
            </form>
        </Shell>
    )
}

function VaultCard({vault, selected, onSelect, onDelete}: {
    vault: VaultInfo
    selected: boolean
    onSelect: () => void
    onDelete: () => void
}) {
    const t = useT()
    const lang = useApp((s) => s.lang)
    const last = vault.lastOpened
        ? new Date(vault.lastOpened).toLocaleDateString(lang === 'en' ? 'en-US' : 'pt-BR')
        : '—'
    return (
        <div
            onClick={onSelect}
            className={`flex cursor-pointer items-center gap-3 rounded-xl border px-4 py-3 transition-colors ${
                selected ? 'border-indigo-500/60 bg-accent/10' : 'border-edge bg-input hover:bg-hover'
            }`}
        >
            <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${selected ? 'bg-accent/20 text-accent' : 'bg-white/5 text-mut'}`}>
                <Vault size={18}/>
            </div>
            <div className="min-w-0 flex-1">
                <div className={`truncate text-sm font-semibold ${selected ? 'text-accent' : 'text-soft'}`}>
                    {vault.name}
                </div>
                <div className="text-xs text-faint">{t('auth.lastAccess')}: {last}</div>
            </div>
            <button
                type="button"
                title={t('auth.deleteVault')}
                onClick={(e) => {
                    e.stopPropagation()
                    onDelete()
                }}
                className="text-faint transition-colors hover:text-red-400"
            >
                <Trash2 size={16}/>
            </button>
        </div>
    )
}

export function UnlockScreen() {
    const t = useT()
    const vaults = useApp((s) => s.vaults)
    const unlock = useApp((s) => s.unlock)
    const newVault = useApp((s) => s.newVault)
    const deleteVault = useApp((s) => s.deleteVault)
    const [selectedFile, setSelectedFile] = useState<string | null>(null)
    const [password, setPassword] = useState('')
    const [loading, setLoading] = useState(false)

    const selected = vaults.find((v) => v.file === selectedFile) ?? null

    async function submit() {
        if (!selected) return
        setLoading(true)
        try {
            await unlock(selected.file, password)
        } catch (err) {
            toast.error(await errorMessage(err))
            setPassword('')
        } finally {
            setLoading(false)
        }
    }

    function confirmDelete(vault: VaultInfo) {
        if (confirm(t('auth.confirmDeleteVault', {name: vault.name}))) {
            void deleteVault(vault.file)
        }
    }

    return (
        <Shell>
            {!selected ? (
                <>
                    <h2 className="mb-1 flex items-center gap-2 text-lg font-semibold text-ink">
                        <Vault size={18} className="text-accent"/> {t('auth.yourVaults')}
                    </h2>
                    <p className="mb-5 text-sm text-mut">{t('auth.selectVault')}</p>

                    {vaults.length === 0 ? (
                        <p className="py-4 text-center text-sm text-faint">{t('auth.noVaults')}</p>
                    ) : (
                        <div className="space-y-2">
                            {vaults.map((v) => (
                                <VaultCard
                                    key={v.file}
                                    vault={v}
                                    selected={false}
                                    onSelect={() => setSelectedFile(v.file)}
                                    onDelete={() => confirmDelete(v)}
                                />
                            ))}
                        </div>
                    )}

                    <Button variant="subtle" className="mt-4 w-full" onClick={newVault}>
                        <Plus size={16}/> {t('auth.newVault')}
                    </Button>
                </>
            ) : (
                <>
                    <button
                        onClick={() => {
                            setSelectedFile(null)
                            setPassword('')
                        }}
                        className="mb-3 flex items-center gap-1 text-xs text-mut transition-colors hover:text-ink"
                    >
                        <ChevronLeft size={14}/> {t('auth.allVaults')}
                    </button>
                    <h2 className="mb-1 flex items-center gap-2 text-lg font-semibold text-ink">
                        <KeyRound size={18} className="text-accent"/> {selected.name}
                    </h2>
                    <p className="mb-5 text-sm text-mut">{t('auth.unlockDesc')}</p>

                    <form onSubmit={(e) => {
                        e.preventDefault()
                        void submit()
                    }} className="space-y-4">
                        <Input
                            label={t('auth.masterPw')}
                            type="password"
                            value={password}
                            onChange={setPassword}
                            autoFocus
                            onEnter={() => void submit()}
                        />
                        <LoadingButton loading={loading}>{t('auth.unlock')}</LoadingButton>
                    </form>
                </>
            )}
        </Shell>
    )
}
