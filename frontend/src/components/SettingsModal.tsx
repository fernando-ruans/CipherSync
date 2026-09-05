import {useState} from 'react'
import toast from 'react-hot-toast'
import {Monitor, Moon, Sun} from 'lucide-react'
import {useApp} from '../state'
import {api, errorMessage} from '../lib/api'
import {Button, Input, Modal, RevealInput} from './ui'
import {getStoredTheme, setStoredTheme} from '../lib/theme'
import type {Theme} from '../lib/theme'
import {useT} from '../lib/locales'
import type {Lang} from '../lib/locales'
import {safeCopy} from '../lib/util'

const AUTOLOCK_OPTIONS: {value: number; labelKey: 'settings.never' | 'settings.min1' | 'settings.min5' | 'settings.min15' | 'settings.min30' | 'settings.hour1'}[] = [
    {value: 0, labelKey: 'settings.never'},
    {value: 1, labelKey: 'settings.min1'},
    {value: 5, labelKey: 'settings.min5'},
    {value: 15, labelKey: 'settings.min15'},
    {value: 30, labelKey: 'settings.min30'},
    {value: 60, labelKey: 'settings.hour1'},
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
    const t = useT()
    const [theme, setTheme] = useState<Theme>(getStoredTheme())
    const options: {value: Theme; label: string; icon: React.ReactNode}[] = [
        {value: 'light', label: t('settings.light'), icon: <Sun size={16}/>},
        {value: 'dark', label: t('settings.dark'), icon: <Moon size={16}/>},
        {value: 'system', label: t('settings.system'), icon: <Monitor size={16}/>},
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
    const t = useT()
    const lang = useApp((s) => s.lang)
    const setLang = useApp((s) => s.setLang)
    const autolockMinutes = useApp((s) => s.autolockMinutes)
    const setAutolockMinutes = useApp((s) => s.setAutolockMinutes)
    const closeToTray = useApp((s) => s.closeToTray)
    const setCloseToTray = useApp((s) => s.setCloseToTray)
    const quickAccess = useApp((s) => s.quickAccess)
    const setQuickAccess = useApp((s) => s.setQuickAccess)
    const deleteAccount = useApp((s) => s.deleteAccount)
    const trashDays = useApp((s) => s.trashDays)
    const setTrashDays = useApp((s) => s.setTrashDays)
    const [changePw, setChangePw] = useState(false)
    const [oldPassword, setOldPassword] = useState('')
    const [newPassword, setNewPassword] = useState('')
    const [confirm, setConfirm] = useState('')
    const [loading, setLoading] = useState(false)
    const [confirmingDelete, setConfirmingDelete] = useState(false)
    const [confirmText, setConfirmText] = useState('')
    const [deleting, setDeleting] = useState(false)
    const [backingUp, setBackingUp] = useState(false)
    const [pairCode, setPairCode] = useState('')

    async function installHost() {
        try {
            // extension IDs filled after store publication; empty = permissive manifest
            await api.installNativeHost('', 'ciphersync@ciphersync.app')
            toast.success(t('ext.installed'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function uninstallHost() {
        try {
            await api.uninstallNativeHost()
            toast.success(t('ext.uninstalled'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function genPairing() {
        try {
            setPairCode(await api.generatePairingCode())
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

    async function doBackup() {
        setBackingUp(true)
        try {
            const dest = await api.backupNow()
            toast.success(t('settings.backupSaved', {name: dest.split(/[\\/]/).pop() ?? ''}))
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setBackingUp(false)
        }
    }

    async function changePassword() {
        setLoading(true)
        try {
            await api.changeMasterPassword(oldPassword, newPassword, confirm)
            toast.success(t('settings.pwChanged'))
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
        <Modal title={t('settings.title')} onClose={onClose}>
            <div className="space-y-6">
                <Section title={t('settings.theme')}>
                    <ThemePicker/>
                    <div className="mt-3 flex items-center justify-between gap-3">
                        <span className="text-sm text-soft">Idioma / Language</span>
                        <select
                            value={lang}
                            onChange={(e) => setLang(e.target.value as Lang)}
                            className="rounded-lg border border-edge bg-input px-2 py-1.5 text-sm text-ink outline-none"
                        >
                            <option value="pt-BR">Português (BR)</option>
                            <option value="en">English</option>
                        </select>
                    </div>
                </Section>

                <Section title={t('settings.security')}>
                    <label className="mb-1.5 block text-xs font-medium text-mut">{t('settings.autolock')}</label>
                    <select
                        value={autolockMinutes}
                        onChange={(e) => void setAutolockMinutes(Number(e.target.value)).catch(async (err) => toast.error(await errorMessage(err)))}
                        className="w-full rounded-lg border border-edge bg-input px-3 py-2 text-sm text-ink outline-none"
                    >
                        {AUTOLOCK_OPTIONS.map((o) => (
                            <option key={o.value} value={o.value}>
                                {t(o.labelKey)}
                            </option>
                        ))}
                    </select>
                    <p className="mt-1.5 text-xs text-faint">
                        {t('settings.autolockHint')}
                    </p>

                    <label className="mt-3 flex cursor-pointer items-center justify-between gap-3">
                        <span className="text-sm text-soft">{t('settings.trayClose')}</span>
                        <input
                            type="checkbox"
                            checked={closeToTray}
                            onChange={(e) => void setCloseToTray(e.target.checked).catch(async (err) => toast.error(await errorMessage(err)))}
                            className="h-4 w-4 accent-indigo-500"
                        />
                    </label>

                    <label className="mt-3 flex cursor-pointer items-center justify-between gap-3" title="Ctrl+Shift+Space">
                        <span className="text-sm text-soft">{t('settings.quickAccess')}</span>
                        <input
                            type="checkbox"
                            checked={quickAccess}
                            onChange={(e) => void setQuickAccess(e.target.checked).catch(async (err) => toast.error(await errorMessage(err)))}
                            className="h-4 w-4 accent-indigo-500"
                        />
                    </label>

                    {!changePw ? (
                        <Button variant="subtle" className="mt-3 w-full" onClick={() => setChangePw(true)}>
                            {t('settings.changePw')}
                        </Button>
                    ) : (
                        <div className="mt-3 space-y-3 rounded-lg border border-edge bg-input p-3">
                            <RevealInput label={t('settings.currentPw')} value={oldPassword} onChange={setOldPassword}/>
                            <RevealInput label={t('settings.newPw')} value={newPassword} onChange={setNewPassword}/>
                            <Input label={t('settings.confirmNewPw')} type="password" value={confirm} onChange={setConfirm}/>
                            <div className="flex justify-end gap-2">
                                <Button variant="ghost" onClick={() => setChangePw(false)}>
                                    Cancelar
                                </Button>
                                <Button onClick={() => void changePassword()} disabled={loading}>
                                    {loading ? t('settings.changing') : t('settings.change')}
                                </Button>
                            </div>
                        </div>
                    )}
                </Section>

                <Section title={t('settings.backups')}>
                    <p className="mb-2 text-xs text-faint">
                        {t('settings.backupsHint').split('backups/')[0]}<span className="font-mono">backups/</span>{t('settings.backupsHint').split('backups/')[1] ?? ''}
                    </p>
                    <Button variant="subtle" className="w-full" onClick={() => void doBackup()} disabled={backingUp}>
                        {backingUp ? t('settings.backingUp') : t('settings.backupNow')}
                    </Button>

                    <label className="mb-1.5 mt-3 block text-xs font-medium text-mut">{t('settings.trashKeep')}</label>
                    <select
                        value={trashDays}
                        onChange={(e) => void setTrashDays(Number(e.target.value)).catch(async (err) => toast.error(await errorMessage(err)))}
                        className="w-full rounded-lg border border-edge bg-input px-3 py-2 text-sm text-ink outline-none"
                    >
                        {[7, 14, 30, 60, 90].map((d) => (
                            <option key={d} value={d}>
                                {d} {t('settings.days')}
                            </option>
                        ))}
                    </select>
                </Section>

                <Section title={t('ext.title')}>
                    <p className="mb-2 text-xs text-faint">
                        {t('ext.desc')}
                    </p>
                    <div className="flex gap-2">
                        <Button variant="subtle" className="flex-1" onClick={() => void installHost()}>
                            {t('ext.install')}
                        </Button>
                        <Button variant="ghost" onClick={() => void uninstallHost()}>
                            {t('ext.uninstall')}
                        </Button>
                    </div>
                    <Button variant="subtle" className="mt-2 w-full" onClick={() => void genPairing()}>
                        {t('ext.pairing')}
                    </Button>
                    {pairCode && (
                        <div
                            className="mt-2 cursor-pointer rounded-lg border border-edge bg-input p-3 text-center font-mono text-lg font-bold tracking-widest text-accent"
                            title={t('ext.codeCopied')}
                            onClick={() => void safeCopy(pairCode, t('ext.codeCopied'))}
                        >
                            {pairCode}
                        </div>
                    )}
                    {pairCode && <p className="mt-1.5 text-xs text-faint">{t('ext.pairingDesc')}</p>}
                </Section>

                <Section title={t('settings.about')}>
                    <p className="text-sm text-soft">
                        <span className="font-semibold text-ink">CipherSync</span> v0.4.0 — {t('settings.aboutText')}
                    </p>
                </Section>

                <Section title={t('settings.danger')}>
                    <p className="mb-2 text-xs text-faint">
                        {t('settings.dangerDesc')}
                    </p>
                    {!confirmingDelete ? (
                        <Button
                            variant="danger"
                            className="w-full border border-red-500/40 bg-red-500/10 text-red-400 hover:bg-red-500/20"
                            onClick={() => setConfirmingDelete(true)}
                        >
                            {t('settings.deleteAccount')}
                        </Button>
                    ) : (
                        <div className="space-y-3 rounded-lg border border-red-500/30 bg-red-500/5 p-3">
                            <p className="text-xs text-red-300">
                                {t('settings.deleteConfirm').split('DELETAR TUDO')[0]}<span className="font-bold">DELETAR TUDO</span>{t('settings.deleteConfirm').split('DELETAR TUDO')[1] ?? ''}
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
                                    {deleting ? t('settings.deleting') : t('settings.deleteForever')}
                                </Button>
                            </div>
                        </div>
                    )}
                </Section>
            </div>
        </Modal>
    )
}
