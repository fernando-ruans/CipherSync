import {useEffect, useRef, useState} from 'react'
import toast from 'react-hot-toast'
import {Camera, CheckCircle2, ImagePlus, KeyRound, Loader2} from 'lucide-react'
import jsQR from 'jsqr'
import {api, errorMessage} from '../lib/api'
import {useT} from '../lib/locales'
import {Button, Input, Modal} from './ui'

type Tab = 'camera' | 'upload' | 'manual'

function CodePreview({secret, onSave}: {secret: string; onSave: (s: string) => void}) {
    const t = useT()
    const [code, setCode] = useState('')
    const [remaining, setRemaining] = useState(0)
    const [saving, setSaving] = useState(false)

    useEffect(() => {
        let alive = true
        const tick = async () => {
            try {
                const c = await api.getTOTPCodeForSecret(secret)
                if (alive) {
                    setCode(c.code)
                    setRemaining(c.secondsRemaining)
                }
            } catch {
                // ignore
            }
        }
        void tick()
        const id = setInterval(tick, 1000)
        return () => {
            alive = false
            clearInterval(id)
        }
    }, [secret])

    async function save() {
        setSaving(true)
        try {
            await onSave(secret)
        } finally {
            setSaving(false)
        }
    }

    return (
        <div className="rounded-xl border border-edge bg-input p-4">
            <div className="flex items-center justify-between">
                <div>
                    <div className="text-xs font-medium text-mut">{t('totp.detected')}</div>
                    <div className="mt-1 font-mono text-sm text-accent">{secret}</div>
                </div>
                <div className="text-center">
                    <div className="font-mono text-3xl font-bold tracking-widest text-ink">{code}</div>
                    <div className="text-[11px] text-faint">{t('totp.updatesIn', {n: remaining})}</div>
                </div>
            </div>
            <Button className="mt-4 w-full" onClick={() => void save()} disabled={saving}>
                {saving ? <Loader2 size={16} className="animate-spin"/> : <CheckCircle2 size={16}/>} {t('totp.save2fa')}
            </Button>
        </div>
    )
}

export function TOTPSetupModal({onClose, onSave}: {
    onClose: () => void
    onSave: (secret: string) => Promise<void>
}) {
    const t = useT()
    const [tab, setTab] = useState<Tab>('camera')
    const [secret, setSecret] = useState('')
    const videoRef = useRef<HTMLVideoElement>(null)
    const streamRef = useRef<MediaStream | null>(null)
    const rafRef = useRef(0)
    const fileRef = useRef<HTMLInputElement>(null)

    useEffect(() => {
        return () => {
            cancelAnimationFrame(rafRef.current)
            streamRef.current?.getTracks().forEach((t) => t.stop())
        }
    }, [])

    async function ingest(uri: string) {
        try {
            const s = await api.ingestTOTPURI(uri)
            setSecret(s)
            cancelAnimationFrame(rafRef.current)
            streamRef.current?.getTracks().forEach((t) => t.stop())
            toast.success(t('totp.qrOk'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    function scanFrame() {
        const video = videoRef.current
        if (video && video.readyState === video.HAVE_ENOUGH_DATA && video.videoWidth > 0) {
            const canvas = document.createElement('canvas')
            canvas.width = video.videoWidth
            canvas.height = video.videoHeight
            const ctx = canvas.getContext('2d')
            if (ctx) {
                ctx.drawImage(video, 0, 0)
                try {
                    const img = ctx.getImageData(0, 0, canvas.width, canvas.height)
                    const qr = jsQR(img.data, img.width, img.height)
                    if (qr) {
                        void ingest(qr.data)
                        return
                    }
                } catch {
                    // ignore
                }
            }
        }
        rafRef.current = requestAnimationFrame(scanFrame)
    }

    useEffect(() => {
        if (tab !== 'camera' || secret) return
        let cancelled = false
        async function start() {
            try {
                const stream = await navigator.mediaDevices.getUserMedia({
                    video: {facingMode: 'environment'},
                })
                if (cancelled) {
                    stream.getTracks().forEach((t) => t.stop())
                    return
                }
                streamRef.current = stream
                if (videoRef.current) {
                    videoRef.current.srcObject = stream
                    await videoRef.current.play()
                }
                rafRef.current = requestAnimationFrame(scanFrame)
            } catch {
                toast.error(t('totp.noCamera'))
                setTab('upload')
            }
        }
        void start()
        return () => {
            cancelled = true
            cancelAnimationFrame(rafRef.current)
            streamRef.current?.getTracks().forEach((t) => t.stop())
            streamRef.current = null
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [tab, secret])

    async function onFile(file: File) {
        const url = URL.createObjectURL(file)
        const img = new Image()
        img.onload = () => {
            const canvas = document.createElement('canvas')
            canvas.width = img.width
            canvas.height = img.height
            const ctx = canvas.getContext('2d')
            if (!ctx) return
            ctx.drawImage(img, 0, 0)
            const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height)
            const qr = jsQR(imgData.data, canvas.width, canvas.height)
            URL.revokeObjectURL(url)
            if (!qr) {
                toast.error(t('totp.noQR'))
                return
            }
            void ingest(qr.data)
        }
        img.onerror = () => {
            URL.revokeObjectURL(url)
            toast.error(t('totp.badImage'))
        }
        img.src = url
    }

    async function verifyManual() {
        try {
            await api.validateTOTPSecret(secret)
            toast.success(t('totp.validKey'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    const tabs: {value: Tab; label: string; icon: React.ReactNode}[] = [
        {value: 'camera', label: t('totp.camera'), icon: <Camera size={14}/>},
        {value: 'upload', label: t('totp.upload'), icon: <ImagePlus size={14}/>},
        {value: 'manual', label: t('totp.manual'), icon: <KeyRound size={14}/>},
    ]

    return (
        <Modal title={t('totp.title')} onClose={onClose}>
            <p className="mb-4 text-sm text-mut">
                {t('totp.desc')}
            </p>

            <div className="mb-4 flex rounded-lg border border-edge bg-input p-1">
                {tabs.map((tb) => (
                    <button
                        key={tb.value}
                        onClick={() => setTab(tb.value)}
                        className={`flex flex-1 items-center justify-center gap-1.5 rounded-md py-1.5 text-sm font-medium transition-colors ${
                            tab === tb.value ? 'bg-indigo-500 text-white' : 'text-mut hover:text-ink'
                        }`}
                    >
                        {tb.icon} {tb.label}
                    </button>
                ))}
            </div>

            {!secret && tab === 'camera' && (
                <div className="relative aspect-square w-full overflow-hidden rounded-xl border border-edge bg-black/40">
                    <video ref={videoRef} muted playsInline className="h-full w-full object-cover"/>
                    <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
                        <div className="h-40 w-40 rounded-xl border-2 border-indigo-400/70"/>
                    </div>
                    <p className="absolute bottom-3 left-0 right-0 text-center text-xs text-white/80">
                        {t('totp.pointCamera')}
                    </p>
                </div>
            )}

            {!secret && tab === 'upload' && (
                <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-edge bg-input py-10">
                    <ImagePlus size={28} className="text-faint"/>
                    <Button variant="subtle" onClick={() => fileRef.current?.click()}>
                        {t('totp.selectShot')}
                    </Button>
                    <input
                        ref={fileRef}
                        type="file"
                        hidden
                        accept="image/*"
                        onChange={(e) => {
                            const f = e.target.files?.[0]
                            if (f) void onFile(f)
                            e.target.value = ''
                        }}
                    />
                </div>
            )}

            {!secret && tab === 'manual' && (
                <div className="space-y-3">
                    <Input
                        label={t('totp.secretLabel')}
                        value={secret}
                        onChange={setSecret}
                        placeholder={t('totp.secretPh')}
                    />
                    <Button variant="subtle" className="w-full" onClick={() => void verifyManual()} disabled={!secret.trim()}>
                        {t('totp.verify')}
                    </Button>
                </div>
            )}

            {secret && <CodePreview secret={secret} onSave={onSave}/>}

            <div className="mt-4 flex justify-end">
                <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
            </div>
        </Modal>
    )
}
