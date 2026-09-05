import {useEffect, useState} from 'react'
import toast from 'react-hot-toast'
import {Copy, Dices, RefreshCw} from 'lucide-react'
import {api, errorMessage} from '../lib/api'
import {safeCopy} from '../lib/util'
import {useT} from '../lib/locales'
import {Button, IconButton, Modal} from './ui'
import type {PasswordOptions} from '../lib/types'

const defaultOpts: PasswordOptions = {
    length: 20,
    useUpper: true,
    useLower: true,
    useDigits: true,
    useSymbols: true,
    excludeAmbiguous: false,
}

const pinOpts: PasswordOptions = {
    length: 4,
    useUpper: false,
    useLower: false,
    useDigits: true,
    useSymbols: false,
    excludeAmbiguous: true,
}

export function GeneratorModal({
    onClose,
    onUse,
    preset = 'password',
}: {
    onClose: () => void
    onUse: (value: string) => void
    preset?: 'password' | 'pin'
}) {
    const [tab, setTab] = useState<'char' | 'words'>('char')
    const [opts, setOpts] = useState<PasswordOptions>(preset === 'pin' ? pinOpts : defaultOpts)
    const [words, setWords] = useState(4)
    const [value, setValue] = useState('')
    const [loading, setLoading] = useState(false)
    const t = useT()

    async function generate() {
        setLoading(true)
        try {
            const v = tab === 'char' ? await api.generatePassword(opts) : await api.generatePassphrase(words)
            setValue(v)
        } catch (err) {
            toast.error(await errorMessage(err))
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        void generate()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    useEffect(() => {
        if (!value) return
        void generate()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [opts.length, opts.useUpper, opts.useLower, opts.useDigits, opts.useSymbols, opts.excludeAmbiguous, words, tab])

    async function copy() {
        await safeCopy(value)
    }

    return (
        <Modal title={t('gen.title')} onClose={onClose}>
            <div className="mb-4 flex rounded-lg border border-edge bg-input p-1">
                {(['char', 'words'] as const).map((tabKey) => (
                    <button
                        key={tabKey}
                        onClick={() => setTab(tabKey)}
                        className={`flex-1 rounded-md py-1.5 text-sm font-medium transition-colors ${
                            tab === tabKey ? 'bg-indigo-500 text-white' : 'text-mut hover:text-ink'
                        }`}
                    >
                        {tabKey === 'char' ? t('gen.random') : t('gen.words')}
                    </button>
                ))}
            </div>

            {tab === 'char' ? (
                <div className="space-y-4">
                    <div className="flex gap-2">
                        <button
                            type="button"
                            onClick={() => setOpts({...pinOpts, length: 4})}
                            title={t('gen.pin4Hint')}
                            className="flex-1 rounded-lg border border-edge bg-input px-3 py-2 text-xs font-medium text-mut hover:bg-hover hover:text-ink"
                        >
                            {t('gen.pin4')}
                        </button>
                        <button
                            type="button"
                            onClick={() => setOpts({...pinOpts, length: 6})}
                            title={t('gen.pin6Hint')}
                            className="flex-1 rounded-lg border border-edge bg-input px-3 py-2 text-xs font-medium text-mut hover:bg-hover hover:text-ink"
                        >
                            {t('gen.pin6')}
                        </button>
                        <button
                            type="button"
                            onClick={() => setOpts(defaultOpts)}
                            title={t('gen.strongHint')}
                            className="flex-1 rounded-lg border border-edge bg-input px-3 py-2 text-xs font-medium text-mut hover:bg-hover hover:text-ink"
                        >
                            {t('gen.strong')}
                        </button>
                    </div>
                    <div>
                        <div className="mb-1.5 flex justify-between text-xs">
                            <span className="font-medium text-mut">{t('gen.length')}</span>
                            <span className="font-semibold text-ink">{opts.length}</span>
                        </div>
                        <input
                            type="range"
                            min={4}
                            max={64}
                            value={opts.length}
                            onChange={(e) => setOpts({...opts, length: Number(e.target.value)})}
                            className="w-full"
                        />
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                        {(
                            [
                                ['useUpper', t('gen.upper')],
                                ['useLower', t('gen.lower')],
                                ['useDigits', t('gen.digits')],
                                ['useSymbols', t('gen.symbols')],
                            ] as const
                        ).map(([key, label]) => (
                            <label
                                key={key}
                                className="flex cursor-pointer items-center gap-2 rounded-lg border border-edge bg-input px-3 py-2 text-sm text-soft"
                            >
                                <input
                                    type="checkbox"
                                    checked={opts[key]}
                                    onChange={(e) => setOpts({...opts, [key]: e.target.checked})}
                                    className="accent-indigo-500"
                                />
                                {label}
                            </label>
                        ))}
                    </div>
                    <label className="flex cursor-pointer items-center gap-2 text-sm text-soft">
                        <input
                            type="checkbox"
                            checked={opts.excludeAmbiguous}
                            onChange={(e) => setOpts({...opts, excludeAmbiguous: e.target.checked})}
                            className="accent-indigo-500"
                        />
                        {t('gen.noAmbiguous')}
                    </label>
                </div>
            ) : (
                <div className="space-y-4">
                    <div>
                        <div className="mb-1.5 flex justify-between text-xs">
                            <span className="font-medium text-mut">{t('gen.wordCount')}</span>
                            <span className="font-semibold text-ink">{words}</span>
                        </div>
                        <input
                            type="range"
                            min={3}
                            max={8}
                            value={words}
                            onChange={(e) => setWords(Number(e.target.value))}
                            className="w-full"
                        />
                    </div>
                    <p className="text-xs text-mut">
                        {t('gen.wordsHint')}
                    </p>
                </div>
            )}

            <div className="mt-5 rounded-xl border border-edge bg-input p-3">
                <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0 flex-1">
                        <div className="truncate font-mono text-sm text-accent">{value}</div>
                    </div>
                    <div className="flex shrink-0 gap-1">
                        <IconButton onClick={() => void copy()} title={t('gen.copy')}>
                            <Copy size={15}/>
                        </IconButton>
                        <IconButton onClick={() => void generate()} title={t('gen.regen')}>
                            <RefreshCw size={15} className={loading ? 'animate-spin' : ''}/>
                        </IconButton>
                    </div>
                </div>
            </div>

            <div className="mt-5 flex justify-end gap-2">
                <Button variant="ghost" onClick={onClose}>
                    {t('common.cancel')}
                </Button>
                <Button
                    variant="subtle"
                    onClick={() => void generate()}
                >
                    <Dices size={16}/> {t('common.generate')}
                </Button>
                <Button onClick={() => onUse(value)}>{t('gen.use')}</Button>
            </div>
        </Modal>
    )
}
