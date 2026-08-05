import {useState} from 'react'
import toast from 'react-hot-toast'
import {
    ChevronLeft,
    Fingerprint,
    KeyRound,
    Loader2,
    Lock,
    Plus,
    ShieldCheck,
    Trash2,
    Vault,
} from 'lucide-react'
import {useApp} from '../state'
import {api, errorMessage} from '../lib/api'
import {Button, Input, RevealInput, StrengthMeter} from './ui'
import type {VaultInfo} from '../lib/types'

function Shell({children}: {children: React.ReactNode}) {
    return (
        <div className="flex h-full items-center justify-center bg-[radial-gradient(ellipse_at_top,rgba(99,102,241,0.15),transparent_60%)]">
            <div className="w-full max-w-sm">
                <div className="mb-8 flex flex-col items-center text-center">
                    <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-violet-600 shadow-xl shadow-indigo-500/30">
                        <Lock size={32} className="text-white"/>
                    </div>
                    <h1 className="text-2xl font-bold tracking-tight text-ink">CipherSync</h1>
                    <p className="mt-1 text-sm text-mut">Seu cofre de senhas, criptografado.</p>
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
            <h2 className="mb-1 text-lg font-semibold text-ink">Criar seu cofre</h2>
            <p className="mb-5 text-sm text-mut">
                Ele é protegido pela sua senha mestra. <span className="text-soft">Não é possível recuperá-la.</span>
            </p>
            <form onSubmit={(e) => {
                e.preventDefault()
                void submit()
            }} className="space-y-4">
                <Input label="Nome do cofre" value={name} onChange={setName} placeholder="e.g. Pessoal, Trabalho, Família" autoFocus/>
                <div>
                    <RevealInput label="Senha mestra" value={password} onChange={setPassword}/>
                    <StrengthMeter password={password}/>
                </div>
                <Input
                    label="Confirme a senha"
                    type="password"
                    value={confirm}
                    onChange={setConfirm}
                    onEnter={() => void submit()}
                />
                {password.length < 8 && (
                    <p className="flex items-center gap-1.5 text-xs text-amber-400">
                        <ShieldCheck size={14}/> Use pelo menos 8 caracteres.
                    </p>
                )}
                <LoadingButton loading={loading}>Criar cofre</LoadingButton>
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
    const last = vault.lastOpened ? new Date(vault.lastOpened).toLocaleDateString('pt-BR') : 'nunca'
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
                <div className="text-xs text-faint">Último acesso: {last}</div>
            </div>
            <button
                type="button"
                title="Excluir cofre"
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
    const vaults = useApp((s) => s.vaults)
    const unlock = useApp((s) => s.unlock)
    const unlockWithHello = useApp((s) => s.unlockWithHello)
    const newVault = useApp((s) => s.newVault)
    const deleteVault = useApp((s) => s.deleteVault)
    const [selectedFile, setSelectedFile] = useState<string | null>(null)
    const [password, setPassword] = useState('')
    const [loading, setLoading] = useState(false)
    const [helloLoading, setHelloLoading] = useState(false)

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

    async function doHello() {
        if (!selected) return
        setHelloLoading(true)
        try {
            const ok = await unlockWithHello(selected.file)
            if (!ok) toast.error('Windows Hello não está configurado para este cofre')
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setHelloLoading(false)
        }
    }

    function confirmDelete(vault: VaultInfo) {
        if (confirm(`Excluir o cofre "${vault.name}"? Todos os itens serão apagados permanentemente.`)) {
            void deleteVault(vault.file)
        }
    }

    return (
        <Shell>
            {!selected ? (
                <>
                    <h2 className="mb-1 flex items-center gap-2 text-lg font-semibold text-ink">
                        <Vault size={18} className="text-accent"/> Seus cofres
                    </h2>
                    <p className="mb-5 text-sm text-mut">Selecione um cofre para desbloquear.</p>

                    {vaults.length === 0 ? (
                        <p className="py-4 text-center text-sm text-faint">Nenhum cofre ainda.</p>
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
                        <Plus size={16}/> Criar novo cofre
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
                        <ChevronLeft size={14}/> Todos os cofres
                    </button>
                    <h2 className="mb-1 flex items-center gap-2 text-lg font-semibold text-ink">
                        <KeyRound size={18} className="text-accent"/> {selected.name}
                    </h2>
                    <p className="mb-5 text-sm text-mut">Digite sua senha mestra para desbloquear.</p>

                    {selected.helloEnabled && (
                        <Button variant="subtle" className="mb-3 w-full" onClick={() => void doHello()} disabled={helloLoading}>
                            {helloLoading ? <Loader2 size={16} className="animate-spin"/> : <Fingerprint size={16}/>}
                            Desbloquear com Windows Hello
                        </Button>
                    )}

                    <form onSubmit={(e) => {
                        e.preventDefault()
                        void submit()
                    }} className="space-y-4">
                        <Input
                            label="Senha mestra"
                            type="password"
                            value={password}
                            onChange={setPassword}
                            autoFocus
                            onEnter={() => void submit()}
                        />
                        <LoadingButton loading={loading}>Desbloquear</LoadingButton>
                    </form>
                </>
            )}
        </Shell>
    )
}
