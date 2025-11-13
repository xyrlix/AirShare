const DB_NAME = 'airshare'
const DB_VERSION = 1

function open() {
  return new Promise<IDBDatabase>((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains('recvChunks')) db.createObjectStore('recvChunks')
      if (!db.objectStoreNames.contains('recvMeta')) db.createObjectStore('recvMeta')
  if (!db.objectStoreNames.contains('sendState')) db.createObjectStore('sendState')
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export async function putRecvChunk(id: string, index: number, buf: ArrayBuffer) {
  const db = await open()
  const tx = db.transaction('recvChunks', 'readwrite')
  tx.objectStore('recvChunks').put(buf, `${id}:${index}`)
  await tx.done?.catch?.(() => {})
}

export async function getRecvChunkIndices(id: string) {
  const db = await open()
  const tx = db.transaction('recvChunks', 'readonly')
  const store = tx.objectStore('recvChunks')
  const keys: string[] = await new Promise((resolve, reject) => {
    const req = store.getAllKeys()
    req.onsuccess = () => resolve(req.result as string[])
    req.onerror = () => reject(req.error)
  })
  return keys.filter(k => k.startsWith(`${id}:`)).map(k => Number(k.split(':')[1]))
}

export async function assembleRecvFile(id: string, total: number) {
  const db = await open()
  const tx = db.transaction('recvChunks', 'readonly')
  const store = tx.objectStore('recvChunks')
  const parts: BlobPart[] = []
  for (let i = 0; i < total; i++) {
    const key = `${id}:${i}`
    const buf: ArrayBuffer | undefined = await new Promise((resolve, reject) => {
      const req = store.get(key)
      req.onsuccess = () => resolve(req.result as ArrayBuffer | undefined)
      req.onerror = () => reject(req.error)
    })
    if (buf) parts.push(buf)
  }
  return new Blob(parts)
}

export async function clearRecv(id: string) {
  const db = await open()
  const tx = db.transaction('recvChunks', 'readwrite')
  const store = tx.objectStore('recvChunks')
  const keys = await getRecvChunkIndices(id)
  for (const k of keys) store.delete(`${id}:${k}`)
  await tx.done?.catch?.(() => {})
}

export async function putRecvMeta(id: string, meta: any) {
  const db = await open()
  const tx = db.transaction('recvMeta', 'readwrite')
  tx.objectStore('recvMeta').put(meta, id)
  await tx.done?.catch?.(() => {})
}

export async function getRecvMeta(id: string) {
  const db = await open()
  const tx = db.transaction('recvMeta', 'readonly')
  const store = tx.objectStore('recvMeta')
  return await new Promise<any>((resolve, reject) => {
    const req = store.get(id)
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export async function putSendState(id: string, state: any) {
  const db = await open()
  const tx = db.transaction('sendState', 'readwrite')
  tx.objectStore('sendState').put(state, id)
  await tx.done?.catch?.(() => {})
}

export async function getSendState(id: string) {
  const db = await open()
  const tx = db.transaction('sendState', 'readonly')
  const store = tx.objectStore('sendState')
  return await new Promise<any>((resolve, reject) => {
    const req = store.get(id)
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export async function clearSendState(id: string) {
  const db = await open()
  const tx = db.transaction('sendState', 'readwrite')
  tx.objectStore('sendState').delete(id)
}

export function fileKey(name: string, size: number, mtime: number) {
  return `${name}|${size}|${mtime}`
}

export async function putSendAckForFile(fileKeyStr: string, data: { id: string, chunkSize: number, total: number, acked: number[], name: string, size: number }) {
  await putSendState(fileKeyStr, data)
}

export async function getSendAckForFile(fileKeyStr: string) {
  return await getSendState(fileKeyStr)
}

export async function clearSendForFile(fileKeyStr: string) {
  const db = await open()
  const tx = db.transaction('sendState', 'readwrite')
  tx.objectStore('sendState').delete(fileKeyStr)
  await tx.done?.catch?.(() => {})
}
