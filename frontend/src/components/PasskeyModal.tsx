import {useState} from 'react'
import {Button, Input, Modal, RevealInput} from './ui'
import {useT} from '../lib/locales'
import type {PasskeyData} from '../lib/types'

const EMPTY: PasskeyData = {
    credentialId: '',
    rpId: '',
    rpName: '',
    userHandle: '',
    username: '',
    displayName: '',
    privateKey: '',
    publicKey: '',
    coseAlg: -7,
    transports: [],
    aaguid: '',
    backupState: '',
}

const TRANSPORTS = ['internal', 'usb', 'nfc', 'ble', 'hybrid']

export function PasskeyModal({
    initial,
    onClose,
    onSave,
}: {
    initial?: PasskeyData | null
    onClose: () => void
    onSave: (data: PasskeyData) => void
}) {
    const [form, setForm] = useState<PasskeyData>({...EMPTY, ...(initial ?? {})})
    const t = useT()

    function set<K extends keyof PasskeyData>(key: K, value: PasskeyData[K]) {
        setForm((f) => ({...f, [key]: value}))
    }

    function toggleTransport(t: string) {
        setForm((f) => ({
            ...f,
            transports: f.transports.includes(t)
                ? f.transports.filter((x) => x !== t)
                : [...f.transports, t],
        }))
    }

    const valid = form.rpId.trim() !== '' && form.credentialId.trim() !== ''

    return (
        <Modal title={initial ? t('passkey.titleEdit') : t('passkey.titleNew')} onClose={onClose}>
            <p className="mb-4 text-xs text-mut">
                {t('passkey.desc')}
            </p>
            <div className="grid grid-cols-2 gap-3">
                <div className="col-span-2">
                    <Input label={t('passkey.rpId')} value={form.rpId} onChange={(v) => set('rpId', v)} placeholder={t('passkey.rpIdPh')}/>
                </div>
                <Input label={t('passkey.rpName')} value={form.rpName} onChange={(v) => set('rpName', v)} placeholder="GitHub"/>
                <Input label={t('passkey.username')} value={form.username} onChange={(v) => set('username', v)}/>
                <div className="col-span-2">
                    <Input label={t('passkey.credentialId')} value={form.credentialId} onChange={(v) => set('credentialId', v)} placeholder={t('passkey.credPh')}/>
                </div>
                <Input label={t('passkey.userHandle')} value={form.userHandle} onChange={(v) => set('userHandle', v)}/>
                <Input label={t('passkey.displayName')} value={form.displayName} onChange={(v) => set('displayName', v)}/>
                <div className="col-span-2">
                    <RevealInput label={t('passkey.privateKey')} value={form.privateKey} onChange={(v) => set('privateKey', v)} placeholder={t('passkey.privateKeyPh')}/>
                </div>
                <div className="col-span-2">
                    <Input label={t('passkey.publicKey')} value={form.publicKey} onChange={(v) => set('publicKey', v)}/>
                </div>
                <Input label={t('passkey.aaguid')} value={form.aaguid} onChange={(v) => set('aaguid', v)} placeholder={t('passkey.credPh')}/>
                <Input
                    label={t('passkey.coseAlg')}
                    value={String(form.coseAlg)}
                    onChange={(v) => {
                        const n = Number(v)
                        set('coseAlg', Number.isFinite(n) ? n : -7)
                    }}
                    placeholder={t('passkey.algPh')}
                />
            </div>
            <div className="mt-3">
                <span className="mb-1.5 block text-xs font-medium text-mut">{t('passkey.transports')}</span>
                <div className="flex flex-wrap gap-2">
                    {TRANSPORTS.map((tr) => (
                        <button
                            key={tr}
                            type="button"
                            onClick={() => toggleTransport(tr)}
                            className={`rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors ${
                                form.transports.includes(tr)
                                    ? 'border-indigo-500/60 bg-accent/10 text-accent'
                                    : 'border-edge bg-input text-mut hover:text-ink'
                            }`}
                        >
                            {tr}
                        </button>
                    ))}
                </div>
            </div>
            {!form.privateKey.trim() && (
                <p className="mt-3 rounded-lg border border-amber-400/30 bg-amber-400/10 px-3 py-2 text-xs text-amber-400">
                    {t('passkey.noPrivate')}
                </p>
            )}
            <div className="mt-5 flex justify-end gap-2">
                <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
                <Button onClick={() => onSave(form)} disabled={!valid}>{t('common.save')}</Button>
            </div>
        </Modal>
    )
}
