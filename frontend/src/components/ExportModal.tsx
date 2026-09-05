import {useState} from 'react'
import toast from 'react-hot-toast'
import {Download, Loader2} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {Button, Input, Modal} from './ui'
import {downloadFile} from '../lib/util'

type Format = 'csv' | 'json' | 'transfer'

export function ExportModal({onClose}: {onClose: () => void}) {
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
                toast.success('Exportado como CSV')
            } else if (format === 'json') {
                const content = await api.exportJSON()
                downloadFile('ciphersync-export.json', content, 'application/json')
                toast.success('Exportado como JSON')
            } else {
                if (transferPw.length < 8) {
                    toast.error('Use uma senha de pelo menos 8 caracteres')
                    return
                }
                if (transferPw !== confirmPw) {
                    toast.error('As senhas não coincidem')
                    return
                }
                const content = await api.exportEncryptedJSON(transferPw)
                downloadFile('ciphersync-transfer.passapp', content, 'text/plain')
                toast.success('Transferência criptografada gerada')
            }
            onClose()
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    return (
        <Modal title="Exportar itens" onClose={onClose}>
            <div className="space-y-4">
                <div className="flex flex-col gap-2">
                    {(
                        [
                            {value: 'csv', label: 'CSV', hint: 'Legível, abre em qualquer planilha'},
                            {value: 'json', label: 'JSON (sem criptografia)', hint: 'Apenas para uso próprio'},
                            {value: 'transfer', label: 'Transferência CipherSync', hint: 'Criptografado com senha, para outro CipherSync'},
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
                        Aviso: este arquivo contém as senhas em texto puro. Guarde com cuidado.
                    </p>
                )}

                {format === 'transfer' && (
                    <div className="space-y-3">
                        <Input label="Senha de proteção" type="password" value={transferPw} onChange={setTransferPw}/>
                        <Input label="Confirme a senha" type="password" value={confirmPw} onChange={setConfirmPw}/>
                    </div>
                )}

                <Button className="w-full" onClick={() => void doExport()} disabled={loading}>
                    {loading ? <Loader2 size={16} className="animate-spin"/> : <Download size={16}/>} Exportar
                </Button>
            </div>
        </Modal>
    )
}
