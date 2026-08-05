export function extractDomain(rawUrl: string): string {
    try {
        const u = new URL(rawUrl.includes('://') ? rawUrl : 'https://' + rawUrl)
        return u.hostname
    } catch {
        return ''
    }
}

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

