import {create} from 'zustand'
import {api} from './lib/api'
import type {Item, Phase, ItemType} from './lib/types'

interface AppState {
    phase: Phase
    items: Item[]
    selectedId: string | null
    search: string
    category: string
    tag: string
    favoritesOnly: boolean
    favicons: Record<string, string>
    autolockMinutes: number
    defaultType: ItemType
    init: () => Promise<void>
    setup: (password: string, confirm: string) => Promise<void>
    unlock: (password: string) => Promise<void>
    lock: () => Promise<void>
    loadSettings: () => Promise<void>
    setAutolockMinutes: (minutes: number) => Promise<void>
    setSearch: (s: string) => void
    setCategory: (c: string) => void
    setTag: (t: string) => void
    toggleFavoritesOnly: () => void
    selectItem: (id: string | null) => void
    setFavicon: (domain: string, dataUri: string) => void
    createItem: (draft: Partial<Item>) => Promise<Item>
    updateItem: (item: Item) => Promise<void>
    removeItem: (id: string) => Promise<void>
    restoreVersion: (versionId: string) => Promise<void>
    importItems: (items: Item[]) => Promise<import('./lib/types').ImportResult>
    refreshItems: () => Promise<void>
}

export const useApp = create<AppState>((set, get) => ({
    phase: 'loading',
    items: [],
    selectedId: null,
    search: '',
    category: 'all',
    tag: '',
    favoritesOnly: false,
    favicons: {},
    autolockMinutes: 5,
    defaultType: 'login',

    init: async () => {
        try {
            const exists = await api.vaultExists()
            set({phase: exists ? 'unlock' : 'setup'})
        } catch {
            set({phase: 'setup'})
        }
    },

    setup: async (password, confirm) => {
        await api.createVault(password, confirm)
        const items = await api.getItems()
        set({phase: 'main', items, selectedId: null})
    },

    unlock: async (password) => {
        await api.openVault(password)
        const items = await api.getItems()
        set({phase: 'main', items, selectedId: null})
    },

    lock: async () => {
        try {
            await api.lock()
        } finally {
            set({
                phase: 'unlock',
                items: [],
                selectedId: null,
                search: '',
                category: 'all',
                tag: '',
                favoritesOnly: false,
                favicons: {},
            })
        }
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
}))
