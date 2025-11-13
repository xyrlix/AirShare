export async function generateKeyPair() {
  const kp = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveKey'])
  return kp
}

export async function exportPubJwk(key: CryptoKey) {
  return await crypto.subtle.exportKey('jwk', key)
}

export async function importPubJwk(jwk: JsonWebKey) {
  return await crypto.subtle.importKey('jwk', jwk, { name: 'ECDH', namedCurve: 'P-256' }, true, [])
}

export async function deriveAesGcmKey(priv: CryptoKey, pub: CryptoKey) {
  const k = await crypto.subtle.deriveKey({ name: 'ECDH', public: pub }, priv, { name: 'AES-GCM', length: 256 }, true, ['encrypt', 'decrypt'])
  return k
}

export async function aesEncrypt(key: CryptoKey, iv: Uint8Array, data: ArrayBuffer) {
  return await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, data)
}

export async function aesDecrypt(key: CryptoKey, iv: Uint8Array, data: ArrayBuffer) {
  return await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, data)
}

export function makeIv(salt: string, index: number) {
  const enc = new TextEncoder()
  const d = enc.encode(`${salt}:${index}`)
  const h = (self as any).crypto || window.crypto
  const arr = new Uint8Array(12)
  for (let i = 0; i < arr.length; i++) arr[i] = d[i % d.length] ^ (index & 0xff)
  return arr
}
