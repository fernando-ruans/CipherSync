export function extractDomain(rawUrl: string): string {
    try {
        const u = new URL(rawUrl.includes('://') ? rawUrl : 'https://' + rawUrl)
        return u.hostname
    } catch {
        return ''
    }
}

import toast from 'react-hot-toast'
import {getLang} from './langStore'
import {translate, translateError} from './locales'

export function downloadFile(name: string, content: string, mime = 'text/plain') {
    const blob = new Blob([content], {type: mime})
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
}

// safeCopy never rejects: shows a toast and returns success.
export async function safeCopy(text: string, label?: string): Promise<boolean> {
    try {
        await window.go.main.App.CopyToClipboard(text)
        toast.success(label ?? translate(getLang(), 'common.copied'))
        return true
    } catch (e) {
        toast.error(e instanceof Error ? translateError(getLang(), e.message) : translate(getLang(), 'common.copyFailed'))
        return false
    }
}

