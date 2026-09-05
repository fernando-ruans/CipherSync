import {create} from 'zustand'
import {api} from './lib/api'
import type {Item, ItemType, Phase, VaultInfo} from './lib/types'
import {getLang, setLang as persistLang, type Lang} from './lib/langStore'
import {translate} from './lib/locales'

// Module-level dirty guard: ItemDetail registers which item has unsaved
// edits so navigation actions can ask for confirmation first.
let unsavedItemId: string | null = null

export function setUnsavedItem(id: string | null) {
    unsavedItemId = id
}

function confirmDiscard(): boolean {
    return window.confirm(translate(useApp.getState().lang, 'discard.confirm'))
}

interface AppState {
    phase: Phase
    vaults: VaultInfo[]
    vaultName: string
    items: Item[]
    selectedId: string | null
    multiSelected: string[]
    search: string
    category: string
    tag: string
    favoritesOnly: boolean
    trashView: boolean
    trash: Item[]
    trashDays: number
    lang: Lang
    favicons: Record<string, string>
    breachedIds: string[]
    autolockMinutes: number
    closeToTray: boolean
    quickAccess: boolean
    defaultType: ItemType
    init: () => Promise<void>
    newVault: () => void
    setup: (name: string, password: string, confirm: string) => Promise<void>
    unlock: (file: string, password: string) => Promise<void>
    lock: () => Promise<void>
    deleteVault: (file: string) => Promise<void>
    deleteAccount: () => Promise<void>
    loadSettings: () => Promise<void>
    setAutolockMinutes: (minutes: number) => Promise<void>
    setCloseToTray: (enabled: boolean) => Promise<void>
    setQuickAccess: (enabled: boolean) => Promise<void>
    setSearch: (s: string) => void
    setCategory: (c: string) => void
    setTag: (t: string) => void
    toggleFavoritesOnly: () => void
    setTrashView: (v: boolean) => void
    loadTrash: () => Promise<void>
    restoreItem: (id: string) => Promise<void>
    restoreSelected: () => Promise<void>
    purgeSelected: () => Promise<void>
    emptyTrash: () => Promise<void>
    setTrashDays: (days: number) => Promise<void>
    setLang: (lang: Lang) => void
    selectItem: (id: string | null) => void
    setFavicon: (domain: string, dataUri: string) => void
    setBreachedIds: (ids: string[]) => void
    createItem: (draft: Partial<Item>) => Promise<Item>
    updateItem: (item: Item) => Promise<void>
    removeItem: (id: string) => Promise<void>
    restoreVersion: (versionId: string) => Promise<void>
    importItems: (items: Item[]) => Promise<import('./lib/types').ImportResult>
    refreshItems: () => Promise<void>
    toggleMulti: (id: string) => void
    setMulti: (ids: string[], clear?: boolean) => void
    clearMulti: () => void
    deleteSelected: () => Promise<void>
    setCategorySelected: (category: string) => Promise<void>
    addTagSelected: (tag: string) => Promise<void>
    toggleFavoriteSelected: () => Promise<void>
}

export const useApp = create<AppState>((set, get) => ({
    phase: 'loading',
    vaults: [],
    vaultName: '',
    items: [],
    selectedId: null,
    multiSelected: [],
    search: '',
    category: 'all',
    tag: '',
    favoritesOnly: false,
    trashView: false,
    trash: [],
    trashDays: 30,
    lang: getLang(),
    favicons: {},
    breachedIds: [],
    autolockMinutes: 5,
    closeToTray: true,
    quickAccess: true,
    defaultType: 'login',

    init: async () => {
        try {
            const vaults = await api.listVaults()
            set({vaults, phase: vaults.length === 0 ? 'setup' : 'unlock'})
        } catch {
            set({phase: 'setup'})
        }
    },

    newVault: () => {
        set({phase: 'setup'})
    },

    setup: async (name, password, confirm) => {
        await api.createVault(name, password, confirm)
        const items = await api.getItems()
        const vaultName = await api.getCurrentVaultName()
        set({phase: 'main', items, vaultName, selectedId: null})
    },

    unlock: async (file, password) => {
        await api.openVault(file, password)
        const items = await api.getItems()
        const vaultName = await api.getCurrentVaultName()
        set({phase: 'main', items, vaultName, selectedId: null})
    },

    lock: async () => {
        unsavedItemId = null
        try {
            await api.lock()
        } finally {
            set({
                phase: 'unlock',
                vaultName: '',
                items: [],
                selectedId: null,
                multiSelected: [],
                search: '',
                category: 'all',
                tag: '',
                favoritesOnly: false,
                trashView: false,
                trash: [],
                favicons: {},
                breachedIds: [],
            })
        }
    },

    deleteVault: async (file) => {
        await api.deleteVault(file)
        const vaults = await api.listVaults()
        set({vaults, phase: vaults.length === 0 ? 'setup' : 'unlock'})
    },

    deleteAccount: async () => {
        unsavedItemId = null
        await api.deleteAccount()
        set({
            phase: 'setup',
            vaults: [],
            vaultName: '',
            items: [],
            selectedId: null,
            multiSelected: [],
            search: '',
            category: 'all',
            tag: '',
            favoritesOnly: false,
            trashView: false,
            trash: [],
            favicons: {},
            breachedIds: [],
        })
    },

    loadSettings: async () => {
        try {
            const s = await api.getSettings()
            const minutes = parseInt(s['autolock_minutes'] ?? '5', 10)
            const trashDays = parseInt(s['trash_days'] ?? '30', 10)
            set({
                autolockMinutes: isNaN(minutes) ? 5 : minutes,
                defaultType: (s['default_type'] as ItemType) || 'login',
                trashDays: isNaN(trashDays) ? 30 : trashDays,
                closeToTray: (s['close_to_tray'] ?? '1') !== '0',
                quickAccess: (s['quick_access'] ?? '1') !== '0',
            })
        } catch {
            // vault not unlocked yet; ignore
        }
    },

    setAutolockMinutes: async (minutes) => {
        await api.setAutolockMinutes(minutes)
        set({autolockMinutes: minutes})
    },

    setCloseToTray: async (enabled) => {
        await api.setCloseToTray(enabled)
        set({closeToTray: enabled})
    },

    setQuickAccess: async (enabled) => {
        await api.setQuickAccess(enabled)
        set({quickAccess: enabled})
    },

    setTrashDays: async (days) => {
        await api.setTrashDays(days)
        set({trashDays: days})
    },

    setLang: (lang: Lang) => {
        persistLang(lang)
        set({lang})
    },

    setSearch: (search) => set({search}),
    // switching views/filters drops the batch selection so BatchBar can never
    // act on items that are no longer visible
    setCategory: (category) => set({category, tag: '', favoritesOnly: false, trashView: false, multiSelected: []}),
    setTag: (tag) => set({tag, category: 'all', favoritesOnly: false, trashView: false, multiSelected: []}),
    toggleFavoritesOnly: () => set((s) => ({favoritesOnly: !s.favoritesOnly, category: 'all', tag: '', trashView: false, multiSelected: []})),
    setTrashView: (trashView) => {
        if (trashView && unsavedItemId && !confirmDiscard()) {
            return
        }
        unsavedItemId = null
        if (trashView) {
            set({trashView: true, category: 'all', tag: '', favoritesOnly: false, selectedId: null, multiSelected: []})
        } else {
            set({trashView: false, selectedId: null, multiSelected: []})
        }
    },
    loadTrash: async () => {
        const trash = await api.listTrashed()
        set({trash})
    },
    restoreItem: async (id) => {
        await api.restoreTrashed(id)
        const [items, trash] = await Promise.all([api.getItems(), api.listTrashed()])
        // only highlight the restored item when the main list is visible
        set((s) => ({items, trash, selectedId: s.trashView ? null : id}))
    },
    restoreSelected: async () => {
        const ids = [...get().multiSelected]
        if (ids.length === 0) return
        await api.restoreTrashedBatch(ids)
        const [items, trash] = await Promise.all([api.getItems(), api.listTrashed()])
        set((s) => ({
            items,
            trash,
            multiSelected: [],
            selectedId: s.trashView ? null : get().selectedId,
        }))
    },
    purgeSelected: async () => {
        const ids = [...get().multiSelected]
        if (ids.length === 0) return
        await api.purgeTrashed(ids)
        const trash = await api.listTrashed()
        set({trash, multiSelected: [], selectedId: null})
    },
    emptyTrash: async () => {
        const ids = get().trash.map((i) => i.id)
        if (ids.length === 0) return
        await api.purgeTrashed(ids)
        set({trash: [], multiSelected: [], selectedId: null})
    },
    selectItem: (selectedId) => {
        const cur = get().selectedId
        if (selectedId !== cur && unsavedItemId && unsavedItemId === cur) {
            if (!confirmDiscard()) return
        }
        set({selectedId})
    },
    setFavicon: (domain, dataUri) =>
        set((s) => ({favicons: {...s.favicons, [domain]: dataUri}})),
    setBreachedIds: (breachedIds) => set({breachedIds}),

    createItem: async (draft) => {
        const now = Date.now()
        const blank: Item = {
            id: '',
            type: 'login',
            title: translate(get().lang, 'detail.noTitle'),
            username: '',
            password: '',
            url: '',
            notes: '',
            category: '',
            tags: [],
            fields: {},
            totpSecret: '',
            favorite: false,
            deleted: false,
            deletedAt: 0,
            createdAt: now,
            updatedAt: now,
        }
        const item = await api.createItem({...blank, ...draft})
        const items = [...get().items, item].sort((a, b) => a.title.localeCompare(b.title))
        set({items, selectedId: item.id})
        return item
    },

    updateItem: async (item) => {
        await api.updateItem(item)
        // re-sort like the backend does so the list order stays consistent
        const items = get().items.map((i) => (i.id === item.id ? item : i)).sort((a, b) => a.title.localeCompare(b.title))
        set({items})
    },

    removeItem: async (id) => {
        await api.deleteItem(id)
        const [items, trash] = await Promise.all([api.getItems(), api.listTrashed()])
        set({
            items,
            trash,
            selectedId: get().selectedId === id ? null : get().selectedId,
        })
    },

    restoreVersion: async (versionId) => {
        const restored = await api.restoreVersion(versionId)
        const items = get().items.map((i) => (i.id === restored.id ? restored : i))
        set({items, selectedId: restored.id})
    },

    importItems: async (items) => {
        const res = await api.importCommit(items)
        const fresh = await api.getItems()
        set({items: fresh})
        return res
    },

    refreshItems: async () => {
        const items = await api.getItems()
        set((s) => {
            const ids = new Set(items.map((i) => i.id))
            return {
                items,
                selectedId: s.selectedId && ids.has(s.selectedId) ? s.selectedId : null,
                multiSelected: s.multiSelected.filter((id) => ids.has(id)),
            }
        })
    },

    toggleMulti: (id) => {
        set((s) => ({
            multiSelected: s.multiSelected.includes(id)
                ? s.multiSelected.filter((x) => x !== id)
                : [...s.multiSelected, id],
        }))
    },

    setMulti: (ids, clear = false) => {
        if (clear) {
            set({multiSelected: [...ids]})
        } else {
            set((s) => ({multiSelected: [...new Set([...s.multiSelected, ...ids])]}))
        }
    },

    clearMulti: () => set({multiSelected: []}),

    deleteSelected: async () => {
        const ids = [...get().multiSelected]
        if (ids.length === 0) return
        await api.deleteItems(ids)
        const [items, trash] = await Promise.all([api.getItems(), api.listTrashed()])
        set((s) => ({
            items,
            trash,
            multiSelected: [],
            selectedId: s.selectedId && items.some((i) => i.id === s.selectedId) ? s.selectedId : null,
        }))
    },

    setCategorySelected: async (category) => {
        const ids = [...get().multiSelected]
        if (ids.length === 0) return
        await api.setCategoryBatch(ids, category)
        await get().refreshItems()
        set({multiSelected: []})
    },

    addTagSelected: async (tag) => {
        const ids = [...get().multiSelected]
        if (ids.length === 0) return
        await api.addTagBatch(ids, tag)
        await get().refreshItems()
        set({multiSelected: []})
    },

    toggleFavoriteSelected: async () => {
        const ids = [...get().multiSelected]
        if (ids.length === 0) return
        const selected = get().items.filter((i) => ids.includes(i.id))
        const allFav = selected.length > 0 && selected.every((i) => i.favorite)
        await api.setFavoriteBatch(ids, !allFav)
        await get().refreshItems()
        set({multiSelected: []})
    },
}))
