import {useEffect, useMemo, useRef, useState} from 'react'
import toast from 'react-hot-toast'
import {
    AlertTriangle,
    Check,
    Dices,
    Download,
    Folder,
    Globe,
    Layers,
    LogOut,
    Minus,
    Plus,
    RefreshCw,
    RotateCcw,
    Search,
    Settings,
    Shield,
    Star,
    Tag as TagIcon,
    Trash2,
    Undo2,
    Upload,
    X,
} from 'lucide-react'
import {useApp} from '../state'
import {api, errorMessage} from '../lib/api'
import {useT, translate} from '../lib/locales'
import {Button, IconButton, Modal, RevealInput, Input} from './ui'
import {ItemDetail} from './ItemDetail'
import {GeneratorModal} from './GeneratorModal'
import {SettingsModal} from './SettingsModal'
import {ImportModal} from './ImportModal'
import {ExportModal} from './ExportModal'
import {WatchtowerModal} from './WatchtowerModal'
import {SyncModal} from './SyncModal'
import {QuickAccess} from './QuickAccess'
import {useTOTPCode} from './TOTPDisplay'
import {extractDomain, downloadFile, safeCopy} from '../lib/util'
import type {Item} from '../lib/types'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import logo from '../assets/ciphersync-logo-64.png'

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

function Sidebar({onOpenSettings, onOpenImport, onOpenExport, onOpenWatchtower, onOpenSync}: {
    onOpenSettings: () => void
    onOpenImport: () => void
    onOpenExport: () => void
    onOpenWatchtower: () => void
    onOpenSync: () => void
}) {
    const items = useApp((s) => s.items)
    const category = useApp((s) => s.category)
    const tag = useApp((s) => s.tag)
    const favoritesOnly = useApp((s) => s.favoritesOnly)
    const setCategory = useApp((s) => s.setCategory)
    const setTag = useApp((s) => s.setTag)
    const toggleFavoritesOnly = useApp((s) => s.toggleFavoritesOnly)
    const lock = useApp((s) => s.lock)
    const vaultName = useApp((s) => s.vaultName)
    const trashView = useApp((s) => s.trashView)
    const setTrashView = useApp((s) => s.setTrashView)
    const trash = useApp((s) => s.trash)
    const loadTrash = useApp((s) => s.loadTrash)
    const t = useT()

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
            label: t('main.allItems'),
            icon: <Layers size={16}/>,
            active: !favoritesOnly && !trashView && category === 'all' && tag === '',
            count: items.length,
            onClick: () => setCategory('all'),
        },
        {
            key: 'fav',
            label: t('main.favorites'),
            icon: <Star size={16}/>,
            active: favoritesOnly && !trashView,
            count: items.filter((i) => i.favorite).length,
            onClick: () => toggleFavoritesOnly(),
        },
        {
            key: 'watchtower',
            label: t('main.watchtower'),
            icon: <Shield size={16}/>,
            active: false,
            count: null,
            onClick: onOpenWatchtower,
        },
        {
            key: 'sync',
            label: t('sync.title'),
            icon: <RefreshCw size={16}/>,
            active: false,
            count: null,
            onClick: onOpenSync,
        },
        {
            key: 'trash',
            label: t('main.trash'),
            icon: <Trash2 size={16}/>,
            active: trashView,
            count: trash.length,
            onClick: () => {
                void loadTrash().catch(() => undefined)
                setTrashView(true)
            },
        },
    ]

    return (
        <aside className="flex w-56 shrink-0 flex-col border-r border-edge bg-panel">
            <div className="px-4 py-4">
                <div className="flex items-center gap-2">
                    <img src={logo} alt="CipherSync" className="h-9 w-9 rounded-xl object-contain"/>
                    <div className="min-w-0">
                        <div className="text-sm font-bold">
                            <span className="text-ink">Cipher</span>
                            <span style={{color: '#3142cb'}}>Sync</span>
                        </div>
                        <div className="truncate text-[11px] text-faint" title={vaultName || t('main.vaultTitle')}>
                            {vaultName || t('main.vaultTitle')}
                        </div>
                    </div>
                </div>
                <button
                    onClick={() => void lock()}
                    className="mt-3 flex w-full items-center justify-center gap-1.5 rounded-lg border border-edge bg-input px-3 py-1.5 text-xs font-medium text-mut transition-colors hover:bg-hover hover:text-ink"
                >
                    <LogOut size={12}/> {t('main.switchVault')}
                </button>
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
                    {t('main.categories')}
                </div>
                {categories.length === 0 && <div className="px-3 py-2 text-xs text-faint">{t('main.noCategory')}</div>}
                {categories.map(([name, count]) => (
                    <button
                        key={name}
                        onClick={() => setCategory(name)}
                        className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors ${
                            !favoritesOnly && !trashView && category === name && tag === ''
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
                            {t('main.tags')}
                        </div>
                        {tags.map(([name, count]) => (
                            <button
                                key={name}
                                onClick={() => setTag(name)}
                                className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors ${
                                    !favoritesOnly && !trashView && tag === name
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
                    <Upload size={16}/> {t('main.import')}
                </button>
                <button
                    onClick={onOpenExport}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-mut transition-colors hover:bg-hover hover:text-ink"
                >
                    <Download size={16}/> {t('main.export')}
                </button>
                <button
                    onClick={onOpenSettings}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-mut transition-colors hover:bg-hover hover:text-ink"
                >
                    <Settings size={16}/> {t('main.settings')}
                </button>
                <button
                    onClick={() => void lock()}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-mut transition-colors hover:bg-hover hover:text-red-400"
                >
                    <LogOut size={16}/> {t('main.lock')}
                </button>
            </div>
        </aside>
    )
}

function BatchBar({count}: {count: number}) {
    const t = useT()
    const items = useApp((s) => s.items)
    const clearMulti = useApp((s) => s.clearMulti)
    const deleteSelected = useApp((s) => s.deleteSelected)
    const setCategorySelected = useApp((s) => s.setCategorySelected)
    const addTagSelected = useApp((s) => s.addTagSelected)
    const toggleFavoriteSelected = useApp((s) => s.toggleFavoriteSelected)
    const [tagText, setTagText] = useState('')

    const categories = useMemo(
        () => [...new Set(items.map((i) => i.category).filter(Boolean))].sort((a, b) => a.localeCompare(b)),
        [items],
    )

    async function exportSelected(kind: 'csv' | 'json') {
        const ids = useApp.getState().multiSelected
        if (ids.length === 0) return
        try {
            const content = kind === 'csv' ? await api.exportSelectedCSV(ids) : await api.exportSelectedJSON(ids)
            downloadFile(
                `${t('export.selectedFile')}.${kind}`,
                content,
                kind === 'csv' ? 'text/csv' : 'application/json',
            )
            toast.success(t('export.selectedDone'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    const actionClass =
        'inline-flex items-center gap-1.5 rounded-lg border border-edge bg-input px-2.5 py-1.5 text-xs font-medium text-soft transition-colors hover:bg-hover hover:text-ink'

    return (
        <div className="border-b border-edge bg-surface px-3 py-2">
            <div className="mb-2 flex items-center justify-between">
                <span className="text-xs font-semibold text-accent">{count} {t('common.selected')}</span>
                <button
                    onClick={clearMulti}
                    className="flex items-center gap-1 text-xs text-mut transition-colors hover:text-ink"
                >
                    <X size={12}/> {t('common.clear')}
                </button>
            </div>
            <div className="flex flex-wrap items-center gap-1.5">
                <button
                    onClick={() => void deleteSelected()}
                    title={t('main.trash')}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-red-500/30 bg-red-500/10 px-2.5 py-1.5 text-xs font-medium text-red-400 transition-colors hover:bg-red-500/20"
                >
                    <Trash2 size={13}/> {t('main.trash')}
                </button>
                <select
                    defaultValue=""
                    onChange={(e) => {
                        const v = e.target.value
                        if (v) void setCategorySelected(v === '__none__' ? '' : v)
                        e.target.value = ''
                    }}
                    title={t('batch.moveCategory')}
                    className="inline-flex cursor-pointer items-center gap-1.5 rounded-lg border border-edge bg-input px-2.5 py-1.5 text-xs font-medium text-soft outline-none"
                >
                    <option value="" disabled>{t('batch.moveCategory')}</option>
                    {categories.map((c) => (
                        <option key={c} value={c}>{c}</option>
                    ))}
                    <option value="__none__">{t('batch.noCategory')}</option>
                </select>
                <div className="flex items-center rounded-lg border border-edge bg-input">
                    <TagIcon size={13} className="ml-2 text-faint"/>
                    <input
                        value={tagText}
                        onChange={(e) => setTagText(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter' && tagText.trim()) {
                                void addTagSelected(tagText)
                                setTagText('')
                            }
                        }}
                        placeholder={t('batch.tagPh')}
                        title={t('batch.tagPh')}
                        className="w-24 bg-transparent px-2 py-1.5 text-xs text-ink placeholder:text-faint outline-none"
                    />
                </div>
                <button onClick={() => void toggleFavoriteSelected()} className={actionClass}>
                    <Star size={13}/> {t('batch.favorites')}
                </button>
                <button onClick={() => void exportSelected('csv')} className={actionClass}>
                    <Download size={13}/> CSV
                </button>
                <button onClick={() => void exportSelected('json')} className={actionClass}>
                    <Download size={13}/> JSON
                </button>
            </div>
        </div>
    )
}

function ItemList() {
    const t = useT()
    const items = useApp((s) => s.items)
    const selectedId = useApp((s) => s.selectedId)
    const search = useApp((s) => s.search)
    const category = useApp((s) => s.category)
    const tag = useApp((s) => s.tag)
    const favoritesOnly = useApp((s) => s.favoritesOnly)
    const setSearch = useApp((s) => s.setSearch)
    const selectItem = useApp((s) => s.selectItem)
    const createItem = useApp((s) => s.createItem)
    const multiSelected = useApp((s) => s.multiSelected)
    const toggleMulti = useApp((s) => s.toggleMulti)
    const setMulti = useApp((s) => s.setMulti)
    const clearMulti = useApp((s) => s.clearMulti)
    const deleteSelected = useApp((s) => s.deleteSelected)
    const searchRef = useRef<HTMLInputElement>(null)
    const lastIndexRef = useRef(-1)

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

    useEffect(() => {
        function onKey(e: KeyboardEvent) {
            const target = e.target as HTMLElement | null
            const inField = !!target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable)
            const key = e.key.toLowerCase()
            if ((e.ctrlKey || e.metaKey) && key === 'f') {
                e.preventDefault()
                searchRef.current?.focus()
            } else if ((e.ctrlKey || e.metaKey) && key === 'a' && !inField) {
                e.preventDefault()
                if (filtered.length > 0) setMulti(filtered.map((i) => i.id), true)
            } else if ((e.ctrlKey || e.metaKey) && key === 'd') {
                e.preventDefault()
                if (useApp.getState().multiSelected.length > 0) void deleteSelected()
            } else if (e.key === 'Escape') {
                clearMulti()
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [filtered, setMulti, deleteSelected, clearMulti])

    function handleCheckbox(e: React.MouseEvent, index: number) {
        e.stopPropagation()
        const id = filtered[index].id
        if (e.shiftKey && lastIndexRef.current >= 0) {
            const a = Math.min(lastIndexRef.current, index)
            const b = Math.max(lastIndexRef.current, index)
            setMulti(filtered.slice(a, b + 1).map((i) => i.id), true)
        } else {
            toggleMulti(id)
        }
        lastIndexRef.current = index
    }

    function handleRowClick(id: string) {
        selectItem(id)
        clearMulti()
    }

    const allSelected =
        filtered.length > 0 && filtered.every((i) => multiSelected.includes(i.id))
    const someSelected = !allSelected && filtered.some((i) => multiSelected.includes(i.id))

    return (
        <section className="flex w-80 shrink-0 flex-col border-r border-edge bg-panel2">
            {multiSelected.length > 0 ? (
                <BatchBar count={multiSelected.length}/>
            ) : (
                <div className="p-3">
                    <div className="flex items-center gap-2">
                        <div className="relative flex-1">
                            <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-faint"/>
                            <input
                                ref={searchRef}
                                value={search}
                                onChange={(e) => setSearch(e.target.value)}
                                placeholder={t('main.searchPh')}
                                className="w-full rounded-lg border border-edge bg-input py-2 pl-9 pr-3 text-sm text-ink placeholder:text-faint outline-none focus:border-indigo-500/60"
                            />
                        </div>
                        <IconButton title={`${t('main.newItem')} (Ctrl+N)`} onClick={() => void createItem({})}>
                            <Plus size={18}/>
                        </IconButton>
                    </div>
                </div>
            )}

            <div className="flex items-center justify-between border-b border-edge px-3 py-1.5">
                <button
                    onClick={() =>
                        allSelected ? clearMulti() : setMulti(filtered.map((i) => i.id), true)
                    }
                    disabled={filtered.length === 0}
                    className={`flex items-center gap-2 text-xs font-medium transition-colors ${
                        allSelected ? 'text-accent' : 'text-mut hover:text-ink'
                    } disabled:opacity-40`}
                    title={`${t('common.selectAll')} (Ctrl+A)`}
                >
                    <span className="flex h-4 w-4 items-center justify-center rounded border border-edge bg-input">
                        {allSelected ? (
                            <Check size={12}/>
                        ) : someSelected ? (
                            <Minus size={12}/>
                        ) : null}
                    </span>
                    {t('common.selectAll')}
                </button>
                <span className="text-xs text-faint">{filtered.length}</span>
            </div>

            <div className="flex-1 overflow-y-auto px-2 pb-3">
                {filtered.length === 0 ? (
                    <div className="px-3 py-8 text-center text-sm text-faint">{t('main.noItems')}</div>
                ) : (
                    <div className="space-y-0.5">
                        {filtered.map((item, index) => (
                            <ItemRow
                                key={item.id}
                                item={item}
                                index={index}
                                selected={item.id === selectedId}
                                multiChecked={multiSelected.includes(item.id)}
                                onCheckboxClick={(e) => handleCheckbox(e, index)}
                                onRowClick={() => handleRowClick(item.id)}
                            />
                        ))}
                    </div>
                )}
            </div>
        </section>
    )
}

function ItemRow({item, selected, multiChecked, onCheckboxClick, onRowClick}: {
    item: Item
    index: number
    selected: boolean
    multiChecked: boolean
    onCheckboxClick: (e: React.MouseEvent) => void
    onRowClick: () => void
}) {
    const domain = item.type === 'login' && item.url ? extractDomain(item.url) : ''
    const breached = useApp((s) => s.breachedIds.includes(item.id))
    const {code: totp} = useTOTPCode(item.totpSecret || undefined)
    const t = useT()
    return (
        <div
            onClick={onRowClick}
            className={`group flex w-full cursor-pointer items-center gap-2.5 rounded-lg px-3 py-2.5 text-left transition-colors ${
                selected ? 'bg-accent/15' : 'hover:bg-hover'
            }`}
        >
            <button
                type="button"
                onClick={onCheckboxClick}
                title={t('common.selectAll')}
                className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border transition-colors ${
                    multiChecked
                        ? 'border-accent bg-accent text-white'
                        : 'border-edge bg-input text-faint opacity-40 group-hover:opacity-100'
                }`}
            >
                {multiChecked ? <Check size={13}/> : null}
            </button>
            <Favicon url={item.url} domain={domain}/>
            <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-2">
                    <span className={`truncate text-sm font-medium ${selected ? 'text-accent' : 'text-soft'}`}>
                        {item.title || t('detail.noTitle')}
                    </span>
                    <span className="flex shrink-0 items-center gap-1">
                        {totp && (
                            <button
                                type="button"
                                title={t('totp.copyCode')}
                                onClick={(e) => {
                                    e.stopPropagation()
                                    void safeCopy(totp, t('totp.codeCopied'))
                                }}
                                className="font-mono text-xs font-semibold text-accent hover:underline"
                            >
                                {totp.slice(0, 3)} {totp.slice(3)}
                            </button>
                        )}
                        {breached && (
                            <span title={t('watch.breached')} className="text-red-400">
                                <AlertTriangle size={13}/>
                            </span>
                        )}
                        {item.favorite && <Star size={13} className="fill-amber-400 text-amber-400"/>}
                    </span>
                </div>
                <div className="mt-0.5 truncate text-xs text-faint">
                    {item.username || item.category || '—'}
                </div>
            </div>
        </div>
    )
}

function ListPane() {
    const trashView = useApp((s) => s.trashView)
    if (trashView) {
        return <TrashView/>
    }
    return <ItemList/>
}

function TrashView() {
    const trash = useApp((s) => s.trash)
    const loadTrash = useApp((s) => s.loadTrash)
    const restoreItem = useApp((s) => s.restoreItem)
    const restoreSelected = useApp((s) => s.restoreSelected)
    const purgeSelected = useApp((s) => s.purgeSelected)
    const emptyTrash = useApp((s) => s.emptyTrash)
    const multiSelected = useApp((s) => s.multiSelected)
    const toggleMulti = useApp((s) => s.toggleMulti)
    const setMulti = useApp((s) => s.setMulti)
    const clearMulti = useApp((s) => s.clearMulti)
    const lang = useApp((s) => s.lang)
    const t = useT()

    useEffect(() => {
        void loadTrash().catch(() => undefined)
    }, [loadTrash])

    async function doPurge() {
        if (multiSelected.length === 0) return
        if (!confirm(t('trash.purgeConfirm', {n: multiSelected.length}))) return
        try {
            await purgeSelected()
            toast.success(t('trash.purged'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function doEmpty() {
        if (trash.length === 0) return
        if (!confirm(t('trash.emptyConfirm', {n: trash.length}))) return
        try {
            await emptyTrash()
            toast.success(t('trash.emptied'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function doRestore(id: string) {
        try {
            await restoreItem(id)
            toast.success(t('trash.restored'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    async function doRestoreSelected() {
        if (multiSelected.length === 0) return
        try {
            await restoreSelected()
            toast.success(t('trash.restored'))
        } catch (err) {
            toast.error(await errorMessage(err))
        }
    }

    const allSelected = trash.length > 0 && trash.every((i) => multiSelected.includes(i.id))

    return (
        <section className="flex w-80 shrink-0 flex-col border-r border-edge bg-panel2">
            <div className="flex items-center justify-between border-b border-edge px-3 py-2.5">
                <div>
                    <div className="flex items-center gap-1.5 text-sm font-semibold text-ink">
                        <Trash2 size={15} className="text-red-400"/> {t('trash.title')}
                    </div>
                    <div className="mt-0.5 text-xs text-faint">{trash.length} {t('trash.items')}</div>
                </div>
                <button
                    onClick={() => void doEmpty()}
                    disabled={trash.length === 0}
                    className="rounded-lg border border-red-500/30 bg-red-500/10 px-2.5 py-1.5 text-xs font-medium text-red-400 transition-colors hover:bg-red-500/20 disabled:opacity-40"
                >
                    {t('trash.empty')}
                </button>
            </div>

            {multiSelected.length > 0 && (
                <div className="flex items-center gap-1.5 border-b border-edge bg-surface px-3 py-2">
                    <span className="mr-1 text-xs font-semibold text-accent">{multiSelected.length}</span>
                    <button
                        onClick={() => void doRestoreSelected()}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-edge bg-input px-2.5 py-1.5 text-xs font-medium text-soft hover:bg-hover hover:text-ink"
                    >
                        <Undo2 size={13}/> {t('common.restore')}
                    </button>
                    <button
                        onClick={() => void doPurge()}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-red-500/30 bg-red-500/10 px-2.5 py-1.5 text-xs font-medium text-red-400 hover:bg-red-500/20"
                    >
                        <Trash2 size={13}/> {t('trash.purge')}
                    </button>
                </div>
            )}

            <div className="flex items-center justify-between border-b border-edge px-3 py-1.5">
                <button
                    onClick={() => (allSelected ? clearMulti() : setMulti(trash.map((i) => i.id), true))}
                    disabled={trash.length === 0}
                    className="flex items-center gap-2 text-xs font-medium text-mut transition-colors hover:text-ink disabled:opacity-40"
                >
                    <span className="flex h-4 w-4 items-center justify-center rounded border border-edge bg-input">
                        {allSelected ? <Check size={12}/> : null}
                    </span>
                    {t('common.selectAll')}
                </button>
            </div>

            <div className="flex-1 space-y-0.5 overflow-y-auto px-2 pb-3">
                {trash.length === 0 ? (
                    <div className="px-3 py-8 text-center text-sm text-faint">{t('trash.isEmpty')}</div>
                ) : (
                    trash.map((item) => (
                        <div
                            key={item.id}
                            className="group flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 hover:bg-hover"
                        >
                            <button
                                type="button"
                                onClick={() => toggleMulti(item.id)}
                                title={t('common.selectAll')}
                                className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border transition-colors ${
                                    multiSelected.includes(item.id)
                                        ? 'border-accent bg-accent text-white'
                                        : 'border-edge bg-input text-faint opacity-40 group-hover:opacity-100'
                                }`}
                            >
                                {multiSelected.includes(item.id) ? <Check size={13}/> : null}
                            </button>
                            <div className="min-w-0 flex-1">
                                <div className="truncate text-sm font-medium text-soft">{item.title || t('detail.noTitle')}</div>
                                <div className="mt-0.5 text-xs text-faint">
                                    {t('trash.deletedAt')} {item.deletedAt ? new Date(item.deletedAt).toLocaleDateString(lang === 'en' ? 'en-US' : 'pt-BR') : '—'}
                                </div>
                            </div>
                            <button
                                type="button"
                                title={t('trash.restore')}
                                onClick={() => void doRestore(item.id)}
                                className="rounded-lg p-1.5 text-mut opacity-0 transition-all hover:bg-hover hover:text-emerald-400 group-hover:opacity-100"
                            >
                                <RotateCcw size={14}/>
                            </button>
                        </div>
                    ))
                )}
            </div>
        </section>
    )
}

function TopBar({onNew}: {onNew: () => void}) {
    const t = useT()
    const [showGenerator, setShowGenerator] = useState(false)
    const createItem = useApp((s) => s.createItem)

    function useGenerated(value: string) {
        setShowGenerator(false)
        void createItem({password: value, title: t('detail.noTitle')})
    }

    useEffect(() => {
        function onKey(e: KeyboardEvent) {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'g') {
                e.preventDefault()
                setShowGenerator(true)
            }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    }, [])

    return (
        <header className="flex items-center justify-between border-b border-edge px-6 py-3">
            <h1 className="text-sm font-semibold text-soft">{t('main.vaultTitle')}</h1>
            <div className="flex items-center gap-2">
                <Button variant="subtle" onClick={() => setShowGenerator(true)} title={`${t('main.genPw')} (Ctrl+G)`}>
                    <Dices size={16}/> {t('main.genPw')}
                </Button>
                <Button onClick={onNew}>
                    <Plus size={16}/> {t('main.newItem')}
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
    const selectItem = useApp((s) => s.selectItem)
    const [showSettings, setShowSettings] = useState(false)
    const [showImport, setShowImport] = useState(false)
    const [showExport, setShowExport] = useState(false)
    const [showWatchtower, setShowWatchtower] = useState(false)
    const [showSync, setShowSync] = useState(false)

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
                    void safeCopy(sel.password, translate(state.lang, 'qa.copied'))
                }
            }
            if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === 'c') {
                e.preventDefault()
                const state = useApp.getState()
                const sel = state.items.find((i) => i.id === state.selectedId)
                if (sel?.username) {
                    void safeCopy(sel.username, translate(state.lang, 'common.copied'))
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
                    onOpenWatchtower={() => setShowWatchtower(true)}
                    onOpenSync={() => setShowSync(true)}
                />
                <ListPane/>
                <main className="min-w-0 flex-1">
                    <ItemDetail/>
                </main>
            </div>
            {showSettings && <SettingsModal onClose={() => setShowSettings(false)}/>}
            {showImport && <ImportModal onClose={() => setShowImport(false)}/>}
            {showExport && <ExportModal onClose={() => setShowExport(false)}/>}
            {showWatchtower && (
                <WatchtowerModal
                    onClose={() => setShowWatchtower(false)}
                    onSelectItem={(id) => selectItem(id)}
                />
            )}
            {showSync && <SyncModal onClose={() => setShowSync(false)}/>}
            <QuickAccess/>
        </div>
    )
}
