import {useEffect, useState} from 'react'
import toast from 'react-hot-toast'
import {Cloud, FolderSync, Loader2, RefreshCw, Unplug} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {useApp} from '../state'
import {Button, Input, Modal} from './ui'
import {useT} from '../lib/locales'
import type {SyncStatus} from '../lib/types'

type Provider = '' | 'local' | 'drive'

export function SyncModal({onClose}: {onClose: () => void}) {
    const t = useT()
    const refreshItems = useApp((s) => s.refreshItems)
    const loadTrash = useApp((s) => s.loadTrash)
    const [provider, setProvider] = useState<Provider>('')
    const [remote, setRemote] = useState('')
    const [status, setStatus] = useState<SyncStatus | null>(null)
    const [loading, setLoading] = useState(false)
    const [saving, setSaving] = useState(false)
    const [clientId, setClientId] = useState('')
    const [clientSecret, setClientSecret] = useState('')
    const [driveEmail, setDriveEmail] = useState('')

    async function load() {
        try {
            const [cfg, st] = await Promise.all([api.getSyncConfig(), api.getSyncStatus()])
            setProvider((cfg.provider as Provider) || '')
            setRemote(cfg.remote || '')
            setStatus(st)
        } catch {
            // ignore
        }
    }

    useEffect(() => {
        void load()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    async function save() {
        setSaving(true)
        try {
            await api.setSyncConfig(provider, provider === 'local' ? remote.trim() : remote)
            toast.success(t('sync.saved'))
            await load()
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setSaving(false)
        }
    }

    async function disconnect() {
        try {
            if (provider === 'drive') {
                await api.driveDisconnect()
            } else {
                await api.disconnectSync()
            }
            setProvider('')
            setRemote('')
            toast.success(t('sync.disconnected'))
            await load()
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function syncNow() {
        setLoading(true)
        try {
            const res = await api.syncNow()
            await refreshItems()
            await loadTrash().catch(() => undefined)
            await load()
            toast.success(t('sync.done', {res}))
        } catch (err) {
            toast.error(await errorMessage(err))
            await load()
        } finally {
            setLoading(false)
        }
    }

    async function connectDrive() {
        setLoading(true)
        try {
            const email = await api.driveConnect(clientId, clientSecret)
            setDriveEmail(email)
            toast.success(t('sync.driveConnected', {email: email || ''}))
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    async function setupDriveFolder() {
        setLoading(true)
        try {
            const folderId = await api.driveSetupFolder()
            setRemote(folderId)
            setProvider('drive')
            await load()
            toast.success(t('sync.driveFolderOk'))
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    const stateColor =
        status?.state === 'ok' ? 'text-emerald-400'
        : status?.state === 'conflict' ? 'text-amber-400'
        : status?.state === 'error' ? 'text-red-400'
        : 'text-faint'

    return (
        <Modal title={t('sync.title')} onClose={onClose}>
            <div className="space-y-4">
                <div>
                    <span className="mb-1.5 block text-xs font-medium text-mut">{t('sync.provider')}</span>
                    <div className="grid grid-cols-3 gap-2">
                        {(
                            [
                                ['local', t('sync.local'), <FolderSync key="l" size={16}/>],
                                ['drive', 'Google Drive', <Cloud key="d" size={16}/>],
                                ['', t('sync.off'), <Unplug key="o" size={16}/>],
                            ] as const
                        ).map(([v, label, icon]) => (
                            <button
                                key={v}
                                onClick={() => setProvider(v)}
                                className={`flex flex-col items-center gap-1.5 rounded-lg border px-3 py-3 text-xs font-medium transition-colors ${
                                    provider === v
                                        ? 'border-indigo-500/60 bg-accent/10 text-accent'
                                        : 'border-edge bg-input text-mut hover:text-ink'
                                }`}
                            >
                                {icon}
                                {label}
                            </button>
                        ))}
                    </div>
                </div>

                {provider === 'local' && (
                    <Input
                        label={t('sync.folder')}
                        value={remote}
                        onChange={setRemote}
                        placeholder="C:\\Users\\voce\\CipherSync, \\\\NAS\\backup, ..."
                    />
                )}

                {provider === 'drive' && (
                    <div className="space-y-3 rounded-lg border border-edge bg-input p-3">
                        <p className="text-xs text-faint">{t('sync.driveHelp')}</p>
                        <Input label="Google Client ID" value={clientId} onChange={setClientId}/>
                        <Input label="Google Client Secret" type="password" value={clientSecret} onChange={setClientSecret}/>
                        <div className="flex gap-2">
                            <Button variant="subtle" className="flex-1" onClick={() => void connectDrive()} disabled={loading}>
                                {t('sync.driveConnect')}
                            </Button>
                            <Button variant="subtle" className="flex-1" onClick={() => void setupDriveFolder()} disabled={loading}>
                                {t('sync.driveFolder')}
                            </Button>
                        </div>
                        {(driveEmail || remote) && (
                            <p className="text-xs text-mut">
                                {driveEmail && <span className="mr-2">{driveEmail}</span>}
                                {remote && <span className="font-mono text-[11px]">{remote}</span>}
                            </p>
                        )}
                    </div>
                )}

                <div className="flex gap-2">
                    <Button variant="subtle" className="flex-1" onClick={() => void save()} disabled={saving || (provider !== '' && provider !== 'drive' && !remote.trim())}>
                        {t('common.save')}
                    </Button>
                    {provider !== '' && (
                        <Button variant="ghost" onClick={() => void disconnect()}>
                            {t('sync.disconnect')}
                        </Button>
                    )}
                </div>

                <div className="rounded-xl border border-edge bg-input p-4">
                    <div className="flex items-center justify-between">
                        <span className="text-sm font-semibold text-ink">{t('sync.status')}</span>
                        <span className={`text-sm font-bold ${stateColor}`}>
                            {status ? status.state || 'idle' : '—'}
                        </span>
                    </div>
                    {status?.detail && <p className="mt-1 text-xs text-mut">{status.detail}</p>}
                    {status?.conflict && (
                        <p className="mt-1 rounded-lg border border-amber-400/30 bg-amber-400/10 px-3 py-2 text-xs text-amber-400">
                            {status.conflict}
                        </p>
                    )}
                    {status && status.lastSync > 0 && (
                        <p className="mt-1 text-xs text-faint">
                            {t('sync.lastSync')}: {new Date(status.lastSync).toLocaleString()}
                        </p>
                    )}
                    <Button className="mt-3 w-full" onClick={() => void syncNow()} disabled={loading || provider === ''}>
                        {loading ? <Loader2 size={16} className="animate-spin"/> : <RefreshCw size={16}/>}
                        {t('sync.now')}
                    </Button>
                </div>
            </div>
        </Modal>
    )
}
