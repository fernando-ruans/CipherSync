import {useEffect, useState} from 'react'
import toast from 'react-hot-toast'
import {Fingerprint, Monitor, Moon, Sun} from 'lucide-react'
import {useApp} from '../state'
import {api, errorMessage} from '../lib/api'
import {Button, Input, Modal, RevealInput} from './ui'
import {getStoredTheme, setStoredTheme} from '../lib/theme'
import type {Theme} from '../lib/theme'

const AUTOLOCK_OPTIONS = [
    {value: 0, label: 'Nunca'},
    {value: 1, label: '1 minuto'},
    {value: 5, label: '5 minutos'},
    {value: 15, label: '15 minutos'},
    {value: 30, label: '30 minutos'},
    {value: 60, label: '1 hora'},
]

function Section({title, children}: {title: string; children: React.ReactNode}) {
    return (
        <div>
            <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-mut">{title}</h3>
            {children}
        </div>
    )
}

function ThemePicker() {
    const [theme, setTheme] = useState<Theme>(getStoredTheme())
    const options: {value: Theme; label: string; icon: React.ReactNode}[] = [
        {value: 'light', label: 'Claro', icon: <Sun size={16}/>},
        {value: 'dark', label: 'Escuro', icon: <Moon size={16}/>},
        {value: 'system', label: 'Sistema', icon: <Monitor size={16}/>},
    ]
    return (
        <div className="flex gap-2">
            {options.map((o) => (
                <button
                    key={o.value}
                    onClick={() => {
                        setTheme(o.value)
                        setStoredTheme(o.value)
                    }}
                    className={`flex flex-1 flex-col items-center gap-1.5 rounded-lg border px-3 py-3 text-xs font-medium transition-colors ${
                        theme === o.value
                            ? 'border-indigo-500/60 bg-accent/10 text-accent'
                            : 'border-edge bg-input text-mut hover:text-ink'
                    }`}
                >
                    {o.icon}
                    {o.label}
                </button>
            ))}
        </div>
    )
}

export function SettingsModal({onClose}: {onClose: () => void}) {
    const autolockMinutes = useApp((s) => s.autolockMinutes)
    const setAutolockMinutes = useApp((s) => s.setAutolockMinutes)
    const deleteAccount = useApp((s) => s.deleteAccount)
    const [changePw, setChangePw] = useState(false)
    const [oldPassword, setOldPassword] = useState('')
    const [newPassword, setNewPassword] = useState('')
    const [confirm, setConfirm] = useState('')
    const [loading, setLoading] = useState(false)
    const [confirmingDelete, setConfirmingDelete] = useState(false)
    const [confirmText, setConfirmText] = useState('')
    const [deleting, setDeleting] = useState(false)
    const [helloAvailable, setHelloAvailable] = useState(false)
    const [helloEnabled, setHelloEnabled] = useState(false)

    useEffect(() => {
        void (async () => {
            try {
                const avail = await api.isHelloAvailable()
                setHelloAvailable(avail)
                if (avail) setHelloEnabled(await api.isHelloEnabled())
            } catch {
                // ignore
            }
        })()
    }, [])

    async function toggleHello() {
        try {
            if (helloEnabled) {
                await api.disableHello()
                toast.success('Windows Hello desativado')
            } else {
                await api.enableHello()
                toast.success('Windows Hello ativado')
            }
            setHelloEnabled(!helloEnabled)
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function doDeleteAccount() {
        setDeleting(true)
        try {
            await deleteAccount()
            onClose()
        } catch (err) {
            toast.error(await errorMessage(err))
            setDeleting(false)
        }
    }

    async function changePassword() {
        setLoading(true)
        try {
            await api.changeMasterPassword(oldPassword, newPassword, confirm)
            toast.success('Senha mestra alterada')
            setChangePw(false)
            setOldPassword('')
            setNewPassword('')
            setConfirm('')
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    return (
        <Modal title="Configurações" onClose={onClose}>
            <div className="space-y-6">
                <Section title="Tema">
                    <ThemePicker/>
                </Section>

                <Section title="Segurança">
                    <label className="mb-1.5 block text-xs font-medium text-mut">Bloqueio automático</label>
                    <select
                        value={autolockMinutes}
                        onChange={(e) => void setAutolockMinutes(Number(e.target.value)).catch(async (err) => toast.error(await errorMessage(err)))}
                        className="w-full rounded-lg border border-edge bg-input px-3 py-2 text-sm text-ink outline-none"
                    >
                        {AUTOLOCK_OPTIONS.map((o) => (
                            <option key={o.value} value={o.value}>
                                {o.label}
                            </option>
                        ))}
                    </select>
                    <p className="mt-1.5 text-xs text-faint">
                        O cofre bloqueia após o tempo de inatividade. Ao minimizar a janela, bloqueia imediatamente.
                    </p>

                    {!changePw ? (
                        <Button variant="subtle" className="mt-3 w-full" onClick={() => setChangePw(true)}>
                            Alterar senha mestra
                        </Button>
                    ) : (
                        <div className="mt-3 space-y-3 rounded-lg border border-edge bg-input p-3">
                            <RevealInput label="Senha atual" value={oldPassword} onChange={setOldPassword}/>
                            <RevealInput label="Nova senha" value={newPassword} onChange={setNewPassword}/>
                            <Input label="Confirme a nova senha" type="password" value={confirm} onChange={setConfirm}/>
                            <div className="flex justify-end gap-2">
                                <Button variant="ghost" onClick={() => setChangePw(false)}>
                                    Cancelar
                                </Button>
                                <Button onClick={() => void changePassword()} disabled={loading}>
                                    {loading ? 'Alterando...' : 'Confirmar'}
                                </Button>
                            </div>
                        </div>
                    )}
                </Section>

                <Section title="Windows Hello">
                    {helloAvailable ? (
                        <>
                            <p className="mb-2 text-xs text-faint">
                                Desbloqueie o cofre sem digitar a senha, usando as credenciais da sua sessão Windows
                                (biometria ou PIN).
                            </p>
                            <Button
                                variant={helloEnabled ? 'danger' : 'subtle'}
                                className="w-full"
                                onClick={() => void toggleHello()}
                            >
                                <Fingerprint size={16}/>
                                {helloEnabled ? 'Desativar Windows Hello' : 'Ativar Windows Hello'}
                            </Button>
                        </>
                    ) : (
                        <p className="text-sm text-faint">Disponível apenas no Windows.</p>
                    )}
                </Section>

                <Section title="Sobre">
                    <p className="text-sm text-soft">
                        <span className="font-semibold text-ink">CipherSync</span> v0.4.0 — gerenciador de senhas
                        open-source. Dados criptografados com AES-256-GCM + Argon2id.
                    </p>
                </Section>

                <Section title="Zona de perigo">
                    <p className="mb-2 text-xs text-faint">
                        Apaga todos os cofres e itens permanentemente, resetando o aplicativo. Esta ação não pode ser desfeita.
                    </p>
                    {!confirmingDelete ? (
                        <Button
                            variant="danger"
                            className="w-full border border-red-500/40 bg-red-500/10 text-red-400 hover:bg-red-500/20"
                            onClick={() => setConfirmingDelete(true)}
                        >
                            Excluir conta e todos os dados
                        </Button>
                    ) : (
                        <div className="space-y-3 rounded-lg border border-red-500/30 bg-red-500/5 p-3">
                            <p className="text-xs text-red-300">
                                Para confirmar, digite <span className="font-bold">DELETAR TUDO</span> abaixo.
                            </p>
                            <Input
                                value={confirmText}
                                onChange={setConfirmText}
                                placeholder="DELETAR TUDO"
                                autoFocus
                            />
                            <div className="flex justify-end gap-2">
                                <Button variant="ghost" onClick={() => {
                                    setConfirmingDelete(false)
                                    setConfirmText('')
                                }}>
                                    Cancelar
                                </Button>
                                <Button
                                    className="border border-red-500/40 bg-red-500/15 text-red-300 hover:bg-red-500/25"
                                    disabled={confirmText !== 'DELETAR TUDO' || deleting}
                                    onClick={() => void doDeleteAccount()}
                                >
                                    {deleting ? 'Apagando...' : 'Apagar permanentemente'}
                                </Button>
                            </div>
                        </div>
                    )}
                </Section>
            </div>
        </Modal>
    )
}
