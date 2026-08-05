import {useRef, useState} from 'react'
import toast from 'react-hot-toast'
import {FileUp, Loader2} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {useApp} from '../state'
import {Button, Input, Modal} from './ui'
import type {FieldMapping, ImportResult, Item} from '../lib/types'

type Format = 'auto' | 'bitwarden' | 'csv' | 'transfer'

const FIELDS = [
    {value: 'ignore', label: 'Ignorar'},
    {value: 'title', label: 'Título'},
    {value: 'username', label: 'Usuário'},
    {value: 'password', label: 'Senha'},
    {value: 'url', label: 'URL'},
    {value: 'notes', label: 'Notas'},
    {value: 'category', label: 'Categoria'},
]

export function ImportModal({onClose}: {onClose: () => void}) {
    const [format, setFormat] = useState<Format>('auto')
    const [fileName, setFileName] = useState('')
    const [transferPw, setTransferPw] = useState('')
    const [headers, setHeaders] = useState<string[]>([])
    const [mapping, setMapping] = useState<FieldMapping[]>([])
    const [result, setResult] = useState<ImportResult | null>(null)
    const [loading, setLoading] = useState(false)
    const [rawCSV, setRawCSV] = useState('')
    const fileRef = useRef<HTMLInputElement>(null)
    const importItems = useApp((s) => s.importItems)

    async function onFile(file: File) {
        setFileName(file.name)
        setResult(null)
        setLoading(true)
        try {
            const text = await file.text()
            if (format === 'bitwarden') {
                setResult(await api.importBitwardenJSON(text))
            } else if (format === 'auto') {
                setResult(await api.importAutoCSV(text))
            } else if (format === 'transfer') {
                if (!transferPw) {
                    toast.error('Digite a senha do arquivo de transferência')
                    return
                }
                setResult(await api.importEncryptedTransfer(text.trim(), transferPw))
            } else {
                const lines = text.split(/\r?\n/)
                const head = lines[0]?.split(',').map((h, i) => (h.trim() || `Coluna ${i + 1}`))
                setHeaders(head ?? [])
                setMapping(
                    (head ?? []).map((_, i) => ({
                        column: i,
                        field: i === 0 ? 'title' : i === 1 ? 'username' : i === 2 ? 'password' : 'ignore',
                    })),
                )
                setResult(null)
                setRawCSV(text)
            }
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    async function runMappedImport() {
        setLoading(true)
        try {
            const res = await api.importCSV(rawCSV, mapping.filter((m) => m.field !== 'ignore'))
            setResult(res)
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    async function commit() {
        if (!result || result.preview.length === 0) return
        setLoading(true)
        try {
            const res = await importItems(result.preview)
            toast.success(`Importados ${res.created} itens` + (res.skipped ? `, ${res.skipped} ignorados` : ''))
            onClose()
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }
    const formatOptions: {value: Format; label: string; hint: string}[] = [
        {value: 'auto', label: 'CSV (LastPass / 1Password)', hint: 'Detecção automática de colunas'},
        {value: 'bitwarden', label: 'Bitwarden JSON', hint: 'Exportação .json não criptografada'},
        {value: 'csv', label: 'CSV genérico', hint: 'Escolha manualmente o que cada coluna representa'},
        {value: 'transfer', label: 'Transferência LockSync', hint: 'Arquivo .passapp criptografado'},
    ]

    return (
        <Modal title="Importar itens" onClose={onClose}>
            <div className="space-y-4">
                <div className="flex flex-col gap-2">
                    {formatOptions.map((o) => (
                        <button
                            key={o.value}
                            onClick={() => {
                                setFormat(o.value)
                                setResult(null)
                                setHeaders([])
                                setFileName('')
                            }}
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

                {format === 'transfer' && (
                    <Input
                        label="Senha do arquivo de transferência"
                        type="password"
                        value={transferPw}
                        onChange={setTransferPw}
                    />
                )}

                <input
                    ref={fileRef}
                    type="file"
                    hidden
                    accept={format === 'bitwarden' ? '.json' : format === 'transfer' ? '.passapp,.txt' : '.csv,.txt'}
                    onChange={(e) => {
                        const f = e.target.files?.[0]
                        if (f) void onFile(f)
                        e.target.value = ''
                    }}
                />
                <Button variant="subtle" className="w-full" onClick={() => fileRef.current?.click()} disabled={loading}>
                    {loading ? <Loader2 size={16} className="animate-spin"/> : <FileUp size={16}/>}
                    {fileName ? `Selecionado: ${fileName}` : 'Selecionar arquivo'}
                </Button>

                {headers.length > 0 && (
                    <div className="space-y-2">
                        <p className="text-xs font-medium text-mut">Mapear colunas</p>
                        {headers.map((h, i) => (
                            <div key={i} className="flex items-center gap-2">
                                <span className="w-40 truncate text-sm text-soft">{h}</span>
                                <select
                                    value={mapping.find((m) => m.column === i)?.field ?? 'ignore'}
                                    onChange={(e) =>
                                        setMapping((prev) =>
                                            prev.map((m) =>
                                                m.column === i ? {...m, field: e.target.value} : m,
                                            ),
                                        )
                                    }
                                    className="flex-1 rounded-lg border border-edge bg-input px-2 py-1.5 text-sm text-ink outline-none"
                                >
                                    {FIELDS.map((f) => (
                                        <option key={f.value} value={f.value}>
                                            {f.label}
                                        </option>
                                    ))}
                                </select>
                            </div>
                        ))}
                        <Button variant="subtle" className="w-full" onClick={() => void runMappedImport()}>
                            Analisar CSV
                        </Button>
                    </div>
                )}

                {result && (
                    <div className="rounded-lg border border-edge bg-input p-4">
                        <div className="flex items-center justify-between">
                            <p className="text-sm text-soft">
                                {result.preview.length} item(ns) pronto(s) para importar
                            </p>
                            {result.preview.length === 0 && (
                                <p className="text-xs text-faint">Nenhum dado válido encontrado.</p>
                            )}
                        </div>
                        <div className="mt-3 max-h-44 overflow-y-auto space-y-1">
                            {result.preview.slice(0, 50).map((it: Item, i) => (
                                <div key={i} className="flex items-center justify-between rounded px-2 py-1 text-sm hover:bg-hover">
                                    <span className="truncate text-soft">{it.title || 'Sem título'}</span>
                                    <span className="ml-2 shrink-0 text-xs text-faint">{it.username}</span>
                                </div>
                            ))}
                            {result.preview.length > 50 && (
                                <p className="px-2 text-xs text-faint">... e mais {result.preview.length - 50}</p>
                            )}
                        </div>
                        <Button className="mt-3 w-full" onClick={() => void commit()} disabled={loading || result.preview.length === 0}>
                            {loading ? <Loader2 size={16} className="animate-spin"/> : 'Confirmar importação'}
                        </Button>
                    </div>
                )}
            </div>
        </Modal>
    )
}
