export type Theme = 'dark' | 'light' | 'system'

const STORAGE_KEY = 'locksync-theme'

export function getStoredTheme(): Theme {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'dark' || v === 'light' || v === 'system') return v
    return 'system'
}

export function setStoredTheme(t: Theme) {
    localStorage.setItem(STORAGE_KEY, t)
    applyTheme(t)
}

export function applyTheme(t: Theme) {
    const dark = t === 'dark' || (t === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
    document.documentElement.classList.toggle('light', !dark)
    document.documentElement.classList.toggle('dark', dark)
}

export function initTheme() {
    const t = getStoredTheme()
    applyTheme(t)
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
        const current = getStoredTheme()
        if (current === 'system') applyTheme(current)
    })
}
