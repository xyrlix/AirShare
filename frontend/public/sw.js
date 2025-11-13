const CACHE_NAME = 'airshare-cache-v1'
const API_CACHE = 'airshare-api-v1'
let retryTimer = null

function openDb() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open('airshare-sw', 1)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains('reqs')) db.createObjectStore('reqs')
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function putReq(id, obj) {
  const db = await openDb()
  const tx = db.transaction('reqs', 'readwrite')
  tx.objectStore('reqs').put(obj, id)
}

async function getAllReqs() {
  const db = await openDb()
  const tx = db.transaction('reqs', 'readonly')
  const store = tx.objectStore('reqs')
  return await new Promise((resolve, reject) => {
    const r = store.getAll()
    r.onsuccess = () => resolve(r.result)
    r.onerror = () => reject(r.error)
  })
}

async function getAllKeys() {
  const db = await openDb()
  const tx = db.transaction('reqs', 'readonly')
  const store = tx.objectStore('reqs')
  return await new Promise((resolve, reject) => {
    const r = store.getAllKeys()
    r.onsuccess = () => resolve(r.result)
    r.onerror = () => reject(r.error)
  })
}

async function delReq(id) {
  const db = await openDb()
  const tx = db.transaction('reqs', 'readwrite')
  tx.objectStore('reqs').delete(id)
}

self.addEventListener('install', e => {
  e.waitUntil(self.skipWaiting())
})

self.addEventListener('activate', e => {
  e.waitUntil((async () => { await self.clients.claim(); scheduleRetry() })())
})

function scheduleRetry() {
  if (retryTimer) return
  retryTimer = setInterval(processQueue, 10000)
}

async function cachePut(cacheName, req, res) {
  const cache = await caches.open(cacheName)
  await cache.put(req, res)
}

async function cacheMatch(cacheName, req) {
  const cache = await caches.open(cacheName)
  const hit = await cache.match(req)
  return hit || null
}

async function processQueue() {
  try {
    const keys = await getAllKeys()
    const list = await getAllReqs()
    for (let i = 0; i < list.length; i++) {
      const item = list[i]
      const id = keys[i]
      if (!item || !id) continue
      const now = Date.now()
      if (Number(item.nextAt || 0) > now) continue
      try {
        const headers = new Headers(item.headers || {})
        const body = item.body ? new Uint8Array(item.body).buffer : undefined
        const r = await fetch(item.url, { method: item.method, headers, body })
        if (r.ok) { await delReq(id); postQueueStats() }
        else {
          const n = Number(item.tryCount || 0) + 1
          const wait = Math.min(300000, Math.pow(2, n) * 5000)
          item.tryCount = n
          item.nextAt = Date.now() + wait
          await putReq(id, item)
          postQueueStats()
        }
      } catch {
        const n = Number(item.tryCount || 0) + 1
        const wait = Math.min(300000, Math.pow(2, n) * 5000)
        item.tryCount = n
        item.nextAt = Date.now() + wait
        await putReq(id, item)
        postQueueStats()
      }
    }
  } catch {}
}

self.addEventListener('message', e => {
  const d = e.data
  if (d && d.type === 'retry-now') processQueue()
  if (d && d.type === 'get-queue') sendQueueStats()
})

self.addEventListener('fetch', e => {
  const req = e.request
  const url = new URL(req.url)
  if (req.mode === 'navigate') {
    e.respondWith((async () => {
      try {
        const res = await fetch(req)
        const clone = res.clone()
        try { await cachePut(CACHE_NAME, req, clone) } catch {}
        return res
      } catch {
        const hit = await cacheMatch(CACHE_NAME, req)
        if (hit) return hit
        return new Response('<h1>Offline</h1>', { status: 503, headers: { 'Content-Type': 'text/html' } })
      }
    })())
    return
  }
  if (req.method === 'GET' && url.pathname.startsWith('/api/files')) {
    e.respondWith((async () => {
      try {
        const res = await fetch(req)
        if (!req.headers.get('range')) {
          const clone = res.clone()
          try { await cachePut(API_CACHE, req, clone) } catch {}
        }
        return res
      } catch {
        const hit = await cacheMatch(API_CACHE, req)
        if (hit) return hit
        return new Response(JSON.stringify({ error: 'offline' }), { status: 503, headers: { 'Content-Type': 'application/json' } })
      }
    })())
    return
  }
  if (req.method !== 'GET' && url.pathname.startsWith('/api/files')) {
    e.respondWith((async () => {
      try {
        const res = await fetch(req)
        return res
      } catch {
        try {
          const clone = req.clone()
          const body = await clone.arrayBuffer()
          const headers = {}
          for (const [k, v] of clone.headers.entries()) headers[k] = v
          const obj = { url: clone.url, method: clone.method, headers, body, tryCount: 0, nextAt: Date.now() + 10000 }
          const id = makeReqId(obj)
          const exists = await hasReq(id)
          if (!exists) { await putReq(id, obj); postQueueStats() }
          scheduleRetry()
        } catch {}
        return new Response(JSON.stringify({ queued: true }), { status: 202, headers: { 'Content-Type': 'application/json' } })
      }
    })())
    return
  }
  e.respondWith(fetch(req))
})

function makeReqId(obj) {
  const len = obj.body ? obj.body.byteLength || 0 : 0
  return `${obj.method}:${obj.url}:${len}`
}

async function hasReq(id) {
  try {
    const keys = await getAllKeys()
    return keys.includes(id)
  } catch { return false }
}

async function sendQueueStats() {
  try {
    const keys = await getAllKeys()
    const clis = await self.clients.matchAll()
    clis.forEach(c => c.postMessage({ type: 'queue-stats', size: keys.length }))
  } catch {}
}

function postQueueStats() { sendQueueStats() }
