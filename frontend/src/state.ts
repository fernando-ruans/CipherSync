import {create} from 'zustand'
import {api} from './lib/api'
import type {Item, ItemType, Phase, VaultInfo} from './lib/types'

interface AppState {
    phase: Phase
    vaults: VaultInfo[]
    vaultName: string
    vaultFile: string
    items: Item[]
    selectedId: string | null
    multiSelected: string[]
    search: string
    category: string
    tag: string
    favoritesOnly: boolean
    favicons: Record<string, string>
    breachedIds: string[]
    autolockMinutes: number
    defaultType: ItemType
    init: () => Promise<void>
    newVault: () => void
    setup: (name: string, password: string, confirm: string) => Promise<void>
    unlock: (file: string, password: string) => Promise<void>
    unlockWithHello: (file: string) => Promise<boolean>
    lock: () => Promise<void>
    deleteVault: (file: string) => Promise<void>
    deleteAccount: () => Promise<void>
    loadSettings: () => Promise<void>
    setAutolockMinutes: (minutes: number) => Promise<void>
    setSearch: (s: string) => void
    setCategory: (c: string) => void
    setTag: (t: string) => void
    toggleFavoritesOnly: () => void
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
    vaultFile: '',
    items: [],
    selectedId: null,
    multiSelected: [],
    search: '',
    category: 'all',
    tag: '',
    favoritesOnly: false,
    favicons: {},
    breachedIds: [],
    autolockMinutes: 5,
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
        set({phase: 'main', items, vaultName, vaultFile: file, selectedId: null})
    },

    unlockWithHello: async (file) => {
        const ok = await api.helloUnlock(file)
        if (!ok) return false
        const items = await api.getItems()
        const vaultName = await api.getCurrentVaultName()
        set({phase: 'main', items, vaultName, vaultFile: file, selectedId: null})
        return true
    },

    lock: async () => {
        try {
            await api.lock()
        } finally {
            set({
                phase: 'unlock',
                vaultName: '',
                vaultFile: '',
                items: [],
                selectedId: null,
                multiSelected: [],
                search: '',
                category: 'all',
                tag: '',
                favoritesOnly: false,
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
        await api.deleteAccount()
        set({
            phase: 'setup',
            vaults: [],
            vaultName: '',
            vaultFile: '',
            items: [],
            selectedId: null,
            multiSelected: [],
            search: '',
            category: 'all',
            tag: '',
            favoritesOnly: false,
            favicons: {},
        })
    },

    loadSettings: async () => {
        try {
            const s = await api.getSettings()
            const minutes = parseInt(s['autolock_minutes'] ?? '5', 10)
            set({
                autolockMinutes: isNaN(minutes) ? 5 : minutes,
                defaultType: (s['default_type'] as ItemType) || 'login',
            })
        } catch {
            // vault not unlocked yet; ignore
        }
    },

    setAutolockMinutes: async (minutes) => {
        await api.setAutolockMinutes(minutes)
        set({autolockMinutes: minutes})
    },

    setSearch: (search) => set({search}),
    setCategory: (category) => set({category, tag: '', favoritesOnly: false}),
    setTag: (tag) => set({tag, category: 'all', favoritesOnly: false}),
    toggleFavoritesOnly: () => set((s) => ({favoritesOnly: !s.favoritesOnly, category: 'all', tag: ''})),
    selectItem: (selectedId) => set({selectedId}),
    setFavicon: (domain, dataUri) =>
        set((s) => ({favicons: {...s.favicons, [domain]: dataUri}})),
    setBreachedIds: (breachedIds) => set({breachedIds}),

    createItem: async (draft) => {
        const now = Date.now()
        const blank: Item = {
            id: '',
            type: 'login',
            title: 'Novo item',
            username: '',
            password: '',
            url: '',
            notes: '',
            category: '',
            tags: [],
            fields: {},
            totpSecret: '',
            favorite: false,
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
        const items = get().items.map((i) => (i.id === item.id ? item : i))
        set({items})
    },

    removeItem: async (id) => {
        await api.deleteItem(id)
        set({
            items: get().items.filter((i) => i.id !== id),
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
        set({items})
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
        await get().refreshItems()
        set((s) => ({
            multiSelected: [],
            selectedId: s.selectedId && get().items.some((i) => i.id === s.selectedId) ? s.selectedId : null,
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
