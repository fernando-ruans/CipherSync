import {useState} from 'react'
import toast from 'react-hot-toast'
import {Download, Loader2} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {Button, Input, Modal} from './ui'
import {useT} from '../lib/locales'
import {downloadFile} from '../lib/util'

type Format = 'csv' | 'json' | 'transfer'

export function ExportModal({onClose}: {onClose: () => void}) {
    const t = useT()
    const [format, setFormat] = useState<Format>('csv')
    const [transferPw, setTransferPw] = useState('')
    const [confirmPw, setConfirmPw] = useState('')
    const [loading, setLoading] = useState(false)

    async function doExport() {
        setLoading(true)
        try {
            if (format === 'csv') {
                const content = await api.exportCSV()
                downloadFile('ciphersync-export.csv', content, 'text/csv')
                toast.success(t('export.csvDone'))
            } else if (format === 'json') {
                const content = await api.exportJSON()
                downloadFile('ciphersync-export.json', content, 'application/json')
                toast.success(t('export.jsonDone'))
            } else {
                if (transferPw.length < 8) {
                    toast.error(t('export.pwShort'))
                    return
                }
                if (transferPw !== confirmPw) {
                    toast.error(t('export.pwMismatch'))
                    return
                }
                const content = await api.exportEncryptedJSON(transferPw)
                downloadFile('ciphersync-transfer.passapp', content, 'text/plain')
                toast.success(t('export.transferDone'))
            }
            onClose()
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    return (
        <Modal title={t('export.title')} onClose={onClose}>
            <div className="space-y-4">
                <div className="flex flex-col gap-2">
                    {(
                        [
                            {value: 'csv', label: 'CSV', hint: t('export.csvHint')},
                            {value: 'json', label: t('export.jsonLabel'), hint: t('export.jsonHint')},
                            {value: 'transfer', label: t('export.transferLabel'), hint: t('export.transferHint')},
                        ] as const
                    ).map((o) => (
                        <button
                            key={o.value}
                            onClick={() => setFormat(o.value)}
                            className={`rounded-lg border px-3 py-2.5 text-left transition-colors ${
                                format === o.value
                                    ? 'border-indigo-500/60 bg-accent/10'
                                    : 'border-edge bg-input hover:bg-hover'
                            }`}
                        >
                            <div className={`text-sm font-medium ${format === o.value ? 'text-accent' : 'text-soft'}`}>
                                {o.label}
                            </div>
                            <div className="text-xs text-faint">{o.hint}</div>
                        </button>
                    ))}
                </div>

                {format === 'json' && (
                    <p className="rounded-lg border border-amber-400/30 bg-amber-400/10 px-3 py-2 text-xs text-amber-400">
                        {t('export.jsonWarn')}
                    </p>
                )}

                {format === 'transfer' && (
                    <div className="space-y-3">
                        <Input label={t('export.transferPw')} type="password" value={transferPw} onChange={setTransferPw}/>
                        <Input label={t('export.confirmPw')} type="password" value={confirmPw} onChange={setConfirmPw}/>
                    </div>
                )}

                <Button className="w-full" onClick={() => void doExport()} disabled={loading}>
                    {loading ? <Loader2 size={16} className="animate-spin"/> : <Download size={16}/>} {t('export.export')}
                </Button>
            </div>
        </Modal>
    )
}
