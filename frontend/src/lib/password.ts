export function passwordScore(pw: string): number {
    if (!pw) return 0
    let score = 0
    if (pw.length >= 8) score++
    if (pw.length >= 14) score++
    const variety = [
        /[a-z]/.test(pw),
        /[A-Z]/.test(pw),
        /\d/.test(pw),
        /[^a-zA-Z0-9]/.test(pw),
    ].filter(Boolean).length
    if (variety >= 3) score++
    if (variety === 4 && pw.length >= 12) score++
    return Math.min(4, score)
}

export function scoreLabel(score: number): string {
    if (score <= 0) return 'Muito fraca'
    if (score === 1) return 'Fraca'
    if (score === 2) return 'Média'
    if (score === 3) return 'Forte'
    return 'Excelente'
}

export function scoreColor(score: number): string {
    if (score <= 1) return '#ef4444'
    if (score === 2) return '#f59e0b'
    if (score === 3) return '#22c55e'
    return '#10b981'
}
