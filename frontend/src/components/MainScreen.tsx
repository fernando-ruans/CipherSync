import {useEffect, useMemo, useRef, useState} from 'react'
import toast from 'react-hot-toast'
import {
    Dices,
    Download,
    Folder,
    Globe,
    Layers,
    Lock,
    LogOut,
    Plus,
    Search,
    Settings,
    Star,
    Tag as TagIcon,
    Upload,
} from 'lucide-react'
import {useApp} from '../state'
import {api} from '../lib/api'
import {Button, IconButton, Modal, RevealInput, Input} from './ui'
import {ItemDetail} from './ItemDetail'
import {GeneratorModal} from './GeneratorModal'
import {SettingsModal} from './SettingsModal'
import {ImportModal} from './ImportModal'
import {ExportModal} from './ExportModal'
import {extractDomain} from '../lib/util'
import type {Item} from '../lib/types'
import {EventsOn} from '../../wailsjs/runtime/runtime'

function Favicon({url, domain}: {url: string; domain: string}) {
    const favicons = useApp((s) => s.favicons)
    const data = domain ? favicons[domain] : undefined
    if (!data) {
        return (
            <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-input text-faint">
                <Globe size={13}/>
            </span>
        )
    }
    return <img src={data} alt="" className="h-6 w-6 shrink-0 rounded-full bg-transparent object-contain"/>
}

function Sidebar({onOpenSettings, onOpenImport, onOpenExport}: {
    onOpenSettings: () => void
    onOpenImport: () => void
    onOpenExport: () => void
}) {
    const items = useApp((s) => s.items)
    const category = useApp((s) => s.category)
    const tag = useApp((s) => s.tag)
    const favoritesOnly = useApp((s) => s.favoritesOnly)
    const setCategory = useApp((s) => s.setCategory)
    const setTag = useApp((s) => s.setTag)
    const toggleFavoritesOnly = useApp((s) => s.toggleFavoritesOnly)
    const lock = useApp((s) => s.lock)

    const categories = useMemo(() => {
        const map = new Map<string, number>()
        for (const it of items) {
            const c = it.category.trim()
            if (c) map.set(c, (map.get(c) ?? 0) + 1)
        }
        return [...map.entries()].sort((a, b) => a[0].localeCompare(b[0]))
    }, [items])

    const tags = useMemo(() => {
        const map = new Map<string, number>()
        for (const it of items) {
            for (const t of it.tags ?? []) {
                if (t) map.set(t, (map.get(t) ?? 0) + 1)
            }
        }
        return [...map.entries()].sort((a, b) => a[0].localeCompare(b[0]))
    }, [items])

    const nav = [
        {
            key: 'all',
            label: 'Todos os itens',
            icon: <Layers size={16}/>,
            active: !favoritesOnly && category === 'all' && tag === '',
            count: items.length,
            onClick: () => setCategory('all'),
        },
        {
            key: 'fav',
            label: 'Favoritos',
            icon: <Star size={16}/>,
            active: favoritesOnly,
            count: items.filter((i) => i.favorite).length,
            onClick: () => toggleFavoritesOnly(),
        },
    ]

    return (
        <aside className="flex w-56 shrink-0 flex-col border-r border-edge bg-panel">
            <div className="flex items-center gap-2 px-4 py-4">
                <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-violet-600">
                    <Lock size={16} className="text-white"/>
                </div>
                <div>
                    <div className="text-sm font-bold text-ink">LockSync</div>
                    <div className="text-[11px] text-faint">Gerenciador de senhas</div>
                </div>
            </div>

            <nav className="flex-1 space-y-0.5 overflow-y-auto px-3 pb-3">
                {nav.map((n) => (
                    <button
                        key={n.key}
                        onClick={n.onClick}
                        className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors ${
                            n.active ? 'bg-accent/15 text-accent' : 'text-mut hover:bg-hover hover:text-ink'
                        }`}
                    >
                        <span className="flex items-center gap-2.5">{n.icon} {n.label}</span>
                        <span className="text-xs text-faint">{n.count}</span>
                    </button>
                ))}

                <div className="px-3 pb-1 pt-5 text-[11px] font-semibold uppercase tracking-wider text-faint">
                    Categorias
                </div>
                {categories.length === 0 && <div className="px-3 py-2 text-xs text-faint">Nenhuma ainda</div>}
                {categories.map(([name, count]) => (
                    <button
                        key={name}
                        onClick={() => setCategory(name)}
                        className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors ${
                            !favoritesOnly && category === name && tag === ''
                                ? 'bg-accent/15 text-accent'
                                : 'text-mut hover:bg-hover hover:text-ink'
                        }`}
                    >
                        <span className="flex items-center gap-2.5">
                            <Folder size={16}/> <span className="truncate">{name}</span>
                        </span>
                        <span className="text-xs text-faint">{count}</span>
                    </button>
                ))}

                {tags.length > 0 && (
                    <>
                        <div className="px-3 pb-1 pt-5 text-[11px] font-semibold uppercase tracking-wider text-faint">
                            Tags
                        </div>
                        {tags.map(([name, count]) => (
                            <button
                                key={name}
                                onClick={() => setTag(name)}
                                className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors ${
                                    !favoritesOnly && tag === name
                                        ? 'bg-accent/15 text-accent'
                                        : 'text-mut hover:bg-hover hover:text-ink'
                                }`}
                            >
                                <span className="flex items-center gap-2.5">
                                    <TagIcon size={16}/> <span className="truncate">{name}</span>
                                </span>
                                <span className="text-xs text-faint">{count}</span>
                            </button>
                        ))}
                    </>
                )}
            </nav>

            <div className="space-y-1 border-t border-edge p-3">
                <button
                    onClick={onOpenImport}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-mut transition-colors hover:bg-hover hover:text-ink"
                >
                    <Upload size={16}/> Importar
                </button>
                <button
                    onClick={onOpenExport}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-mut transition-colors hover:bg-hover hover:text-ink"
                >
                    <Download size={16}/> Exportar
                </button>
                <button
                    onClick={onOpenSettings}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-mut transition-colors hover:bg-hover hover:text-ink"
                >
                    <Settings size={16}/> Configurações
                </button>
                <button
                    onClick={() => void lock()}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-mut transition-colors hover:bg-hover hover:text-red-400"
                >
                    <LogOut size={16}/> Bloquear
                </button>
            </div>
        </aside>
    )
}

function ItemList() {
    const items = useApp((s) => s.items)
    const selectedId = useApp((s) => s.selectedId)
    const search = useApp((s) => s.search)
    const category = useApp((s) => s.category)
    const tag = useApp((s) => s.tag)
    const favoritesOnly = useApp((s) => s.favoritesOnly)
    const setSearch = useApp((s) => s.setSearch)
    const selectItem = useApp((s) => s.selectItem)
    const createItem = useApp((s) => s.createItem)
    const searchRef = useRef<HTMLInputElement>(null)

    useEffect(() => {
        function onKey(e: KeyboardEvent) {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
                e.preventDefault()
                searchRef.current?.focus()
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [])

    const filtered = useMemo(() => {
        const q = search.trim().toLowerCase()
        return items
            .filter((i) => (favoritesOnly ? i.favorite : true))
            .filter((i) => (category === 'all' ? true : i.category.trim() === category))
            .filter((i) => (tag === '' ? true : (i.tags ?? []).includes(tag)))
            .filter((i) =>
                q === ''
                    ? true
                    : i.title.toLowerCase().includes(q) ||
                      i.username.toLowerCase().includes(q) ||
                      i.url.toLowerCase().includes(q) ||
                      i.notes.toLowerCase().includes(q) ||
                      (i.tags ?? []).some((t) => t.includes(q)) ||
                      Object.values(i.fields ?? {}).some((v) => v.toLowerCase().includes(q)),
            )
    }, [items, search, category, tag, favoritesOnly])

    return (
        <section className="flex w-80 shrink-0 flex-col border-r border-edge bg-panel2">
            <div className="p-3">
                <div className="flex items-center gap-2">
                    <div className="relative flex-1">
                        <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-faint"/>
                        <input
                            ref={searchRef}
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            placeholder="Buscar (Ctrl+F)..."
                            className="w-full rounded-lg border border-edge bg-input py-2 pl-9 pr-3 text-sm text-ink placeholder:text-faint outline-none focus:border-indigo-500/60"
                        />
                    </div>
                    <IconButton title="Novo item (Ctrl+N)" onClick={() => void createItem({})}>
                        <Plus size={18}/>
                    </IconButton>
                </div>
            </div>

            <div className="flex-1 overflow-y-auto px-2 pb-3">
                {filtered.length === 0 ? (
                    <div className="px-3 py-8 text-center text-sm text-faint">Nenhum item encontrado.</div>
                ) : (
                    <div className="space-y-0.5">
                        {filtered.map((item) => (
                            <ItemRow key={item.id} item={item} selected={item.id === selectedId}/>
                        ))}
                    </div>
                )}
            </div>
        </section>
    )
}

function ItemRow({item, selected}: {item: Item; selected: boolean}) {
    const selectItem = useApp((s) => s.selectItem)
    const domain = item.type === 'login' && item.url ? extractDomain(item.url) : ''
    return (
        <button
            onClick={() => selectItem(item.id)}
            className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left transition-colors ${
                selected ? 'bg-accent/15' : 'hover:bg-hover'
            }`}
        >
            <Favicon url={item.url} domain={domain}/>
            <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-2">
                    <span className={`truncate text-sm font-medium ${selected ? 'text-accent' : 'text-soft'}`}>
                        {item.title || 'Sem título'}
                    </span>
                    {item.favorite && <Star size={13} className="shrink-0 fill-amber-400 text-amber-400"/>}
                </div>
                <div className="mt-0.5 truncate text-xs text-faint">
                    {item.username || item.category || '—'}
                </div>
            </div>
        </button>
    )
}

function TopBar({onNew}: {onNew: () => void}) {
    const [showGenerator, setShowGenerator] = useState(false)
    const createItem = useApp((s) => s.createItem)

    function useGenerated(value: string) {
        setShowGenerator(false)
        void createItem({password: value, title: 'Novo item'})
    }

    return (
        <header className="flex items-center justify-between border-b border-edge px-6 py-3">
            <h1 className="text-sm font-semibold text-soft">Cofre de senhas</h1>
            <div className="flex items-center gap-2">
                <Button variant="subtle" onClick={() => setShowGenerator(true)}>
                    <Dices size={16}/> Gerar senha
                </Button>
                <Button onClick={onNew}>
                    <Plus size={16}/> Novo item
                </Button>
            </div>
            {showGenerator && <GeneratorModal onClose={() => setShowGenerator(false)} onUse={useGenerated}/>}
        </header>
    )
}

export function MainScreen() {
    const createItem = useApp((s) => s.createItem)
    const loadSettings = useApp((s) => s.loadSettings)
    const setFavicon = useApp((s) => s.setFavicon)
    const [showSettings, setShowSettings] = useState(false)
    const [showImport, setShowImport] = useState(false)
    const [showExport, setShowExport] = useState(false)

    useEffect(() => {
        void loadSettings()
        void api.prefetchFavicons()
        const off = EventsOn('favicon', (payload: Record<string, string>) => {
            for (const [domain, data] of Object.entries(payload)) {
                setFavicon(domain, data)
            }
        })
        return () => {
            off()
        }
    }, [loadSettings, setFavicon])

    useEffect(() => {
        function onKey(e: KeyboardEvent) {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'l') {
                e.preventDefault()
                void useApp.getState().lock()
            }
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'n') {
                e.preventDefault()
                void createItem({})
            }
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'b') {
                e.preventDefault()
                const state = useApp.getState()
                const sel = state.items.find((i) => i.id === state.selectedId)
                if (sel?.password) {
                    void api.copy(sel.password).then(() => toast.success('Senha copiada'))
                }
            }
            if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === 'c') {
                e.preventDefault()
                const state = useApp.getState()
                const sel = state.items.find((i) => i.id === state.selectedId)
                if (sel?.username) {
                    void api.copy(sel.username).then(() => toast.success('Usuário copiado'))
                }
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [createItem])

    return (
        <div className="flex h-full flex-col">
            <TopBar onNew={() => void createItem({})}/>
            <div className="flex min-h-0 flex-1">
                <Sidebar
                    onOpenSettings={() => setShowSettings(true)}
                    onOpenImport={() => setShowImport(true)}
                    onOpenExport={() => setShowExport(true)}
                />
                <ItemList/>
                <main className="min-w-0 flex-1">
                    <ItemDetail/>
                </main>
            </div>
            {showSettings && <SettingsModal onClose={() => setShowSettings(false)}/>}
            {showImport && <ImportModal onClose={() => setShowImport(false)}/>}
            {showExport && <ExportModal onClose={() => setShowExport(false)}/>}
        </div>
    )
}
