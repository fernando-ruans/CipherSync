export type Lang = 'pt-BR' | 'en'

export function getLang(): Lang {
    try {
        const v = localStorage.getItem('ciphersync-lang')
        if (v === 'en' || v === 'pt-BR') return v
    } catch {
        // ignore
    }
    if (typeof navigator !== 'undefined' && navigator.language?.toLowerCase().startsWith('en')) {
        return 'en'
    }
    return 'pt-BR'
}

export function setLang(lang: Lang) {
    try {
        localStorage.setItem('ciphersync-lang', lang)
    } catch {
        // ignore
    }
}
