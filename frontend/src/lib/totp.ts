const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'

function base32Decode(input: string): Uint8Array {
    const clean = input.toUpperCase().replace(/[^A-Z2-7]/g, '')
    let bits = 0
    let value = 0
    const out: number[] = []
    for (const ch of clean) {
        value = (value << 5) | B32.indexOf(ch)
        bits += 5
        if (bits >= 8) {
            out.push((value >>> (bits - 8)) & 0xff)
            bits -= 8
        }
    }
    return new Uint8Array(out)
}

export async function localTOTP(secret: string, timeMs = Date.now()): Promise<{code: string; remaining: number}> {
    const key = base32Decode(secret)
    if (key.length === 0) throw new Error('chave inválida')
    const counter = Math.floor(timeMs / 1000 / 30)
    const msg = new Uint8Array(8)
    const view = new DataView(msg.buffer)
    view.setUint32(0, Math.floor(counter / 0x100000000))
    view.setUint32(4, counter >>> 0)
    const cryptoKey = await crypto.subtle.importKey('raw', key.buffer as ArrayBuffer, {name: 'HMAC', hash: 'SHA-1'}, false, ['sign'])
    const sig = new Uint8Array(await crypto.subtle.sign('HMAC', cryptoKey, msg.buffer as ArrayBuffer))
    const offset = sig[sig.length - 1] & 0xf
    const bin =
        ((sig[offset] & 0x7f) << 24) |
        ((sig[offset + 1] & 0xff) << 16) |
        ((sig[offset + 2] & 0xff) << 8) |
        (sig[offset + 3] & 0xff)
    const code = String(bin % 1000000).padStart(6, '0')
    const remaining = 30 - Math.floor(timeMs / 1000) % 30
    return {code, remaining}
}
