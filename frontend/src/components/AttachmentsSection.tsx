import {useEffect, useRef, useState} from 'react'
import toast from 'react-hot-toast'
import {Download, FileUp, Loader2, Paperclip, Trash2} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {IconButton} from './ui'
import type {Attachment} from '../lib/types'

function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export function AttachmentsSection({itemId}: {itemId: string}) {
    const [files, setFiles] = useState<Attachment[]>([])
    const [uploading, setUploading] = useState(false)
    const [downloading, setDownloading] = useState<string | null>(null)
    const fileRef = useRef<HTMLInputElement>(null)

    async function load() {
        try {
            setFiles(await api.listAttachments(itemId))
        } catch {
            setFiles([])
        }
    }

    useEffect(() => {
        void load()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [itemId])

    function fileToB64(file: File): Promise<string> {
        return new Promise((resolve, reject) => {
            const reader = new FileReader()
            reader.onload = () => {
                const buf = reader.result as ArrayBuffer
                const bytes = new Uint8Array(buf)
                let bin = ''
                const chunk = 0x8000
                for (let i = 0; i < bytes.length; i += chunk) {
                    bin += String.fromCharCode(...bytes.subarray(i, i + chunk))
                }
                resolve(btoa(bin))
            }
            reader.onerror = () => reject(new Error('falha ao ler o arquivo'))
            reader.readAsArrayBuffer(file)
        })
    }

    async function upload(file: File) {
        if (file.size > 10 * 1024 * 1024) {
            toast.error('Arquivo maior que 10 MB')
            return
        }
        setUploading(true)
        try {
            const b64 = await fileToB64(file)
            const att = await api.addAttachment(itemId, file.name, b64)
            setFiles((prev) => [...prev, att])
            toast.success('Anexo adicionado')
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setUploading(false)
        }
    }

    async function download(att: Attachment) {
        setDownloading(att.id)
        try {
            const payload = await api.getAttachment(att.id)
            const bin = atob(payload.data)
            const bytes = new Uint8Array(bin.length)
            for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
            const blob = new Blob([bytes.buffer as ArrayBuffer], {type: 'application/octet-stream'})
            const url = URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = payload.name
            document.body.appendChild(a)
            a.click()
            document.body.removeChild(a)
            URL.revokeObjectURL(url)
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setDownloading(null)
        }
    }

    async function remove(att: Attachment) {
        if (!confirm(`Remover o anexo "${att.name}" permanentemente?`)) return
        try {
            await api.deleteAttachment(att.id)
            setFiles((prev) => prev.filter((f) => f.id !== att.id))
            toast.success('Anexo removido')
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    return (
        <div>
            <span className="mb-1.5 block text-xs font-medium text-mut">Anexos</span>
            <div className="space-y-1.5 rounded-lg border border-edge bg-input p-2.5">
                {files.length === 0 && (
                    <p className="px-1 py-1 text-xs text-faint">Nenhum anexo. Arquivos ficam criptografados no cofre (máx. 10 MB).</p>
                )}
                {files.map((f) => (
                    <div key={f.id} className="flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-hover">
                        <Paperclip size={14} className="shrink-0 text-faint"/>
                        <span className="min-w-0 flex-1 truncate text-sm text-soft" title={f.name}>{f.name}</span>
                        <span className="shrink-0 text-xs text-faint">{formatSize(f.size)}</span>
                        <IconButton
                            title="Baixar"
                            onClick={() => void download(f)}
                            className="h-7 w-7"
                        >
                            {downloading === f.id ? <Loader2 size={14} className="animate-spin"/> : <Download size={14}/>}
                        </IconButton>
                        <IconButton
                            title="Remover anexo"
                            onClick={() => void remove(f)}
                            className="h-7 w-7 text-red-400 hover:bg-red-500/10"
                        >
                            <Trash2 size={14}/>
                        </IconButton>
                    </div>
                ))}
                <input
                    ref={fileRef}
                    type="file"
                    hidden
                    onChange={(e) => {
                        const f = e.target.files?.[0]
                        if (f) void upload(f)
                        e.target.value = ''
                    }}
                />
                <button
                    type="button"
                    onClick={() => fileRef.current?.click()}
                    disabled={uploading}
                    className="flex w-full items-center justify-center gap-1.5 rounded-lg border border-dashed border-edge px-3 py-2 text-xs font-medium text-mut transition-colors hover:bg-hover hover:text-ink disabled:opacity-50"
                >
                    {uploading ? <Loader2 size={14} className="animate-spin"/> : <FileUp size={14}/>}
                    Adicionar arquivo
                </button>
            </div>
        </div>
    )
}
