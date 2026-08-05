import {useState} from 'react'
import toast from 'react-hot-toast'
import {KeyRound, Loader2, Lock, ShieldCheck} from 'lucide-react'
import {useApp} from '../state'
import {errorMessage} from '../lib/api'
import {Button, Input, RevealInput, StrengthMeter} from './ui'

function Shell({children}: {children: React.ReactNode}) {
    return (
        <div className="flex h-full items-center justify-center bg-[radial-gradient(ellipse_at_top,rgba(99,102,241,0.15),transparent_60%)]">
            <div className="w-full max-w-sm">
                <div className="mb-8 flex flex-col items-center text-center">
                    <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-violet-600 shadow-xl shadow-indigo-500/30">
                        <Lock size={32} className="text-white"/>
                    </div>
                    <h1 className="text-2xl font-bold tracking-tight text-ink">LockSync</h1>
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
    const [password, setPassword] = useState('')
    const [confirm, setConfirm] = useState('')
    const [loading, setLoading] = useState(false)

    async function submit() {
        setLoading(true)
        try {
            await setup(password, confirm)
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    return (
        <Shell>
            <h2 className="mb-1 text-lg font-semibold text-ink">Criar sua senha mestra</h2>
            <p className="mb-5 text-sm text-mut">
                Ela protege todo o cofre. <span className="text-soft">Não é possível recuperá-la.</span>
            </p>
            <form onSubmit={(e) => {
                e.preventDefault()
                void submit()
            }} className="space-y-4">
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

export function UnlockScreen() {
    const unlock = useApp((s) => s.unlock)
    const [password, setPassword] = useState('')
    const [loading, setLoading] = useState(false)

    async function submit() {
        setLoading(true)
        try {
            await unlock(password)
        } catch (err) {
            toast.error(await errorMessage(err))
            setPassword('')
        } finally {
            setLoading(false)
        }
    }

    return (
        <Shell>
            <h2 className="mb-1 flex items-center gap-2 text-lg font-semibold text-ink">
                <KeyRound size={18} className="text-accent"/> Desbloquear cofre
            </h2>
            <p className="mb-5 text-sm text-mut">Digite sua senha mestra para continuar.</p>
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
        </Shell>
    )
}
