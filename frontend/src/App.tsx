import React, { useEffect, useMemo, useState } from 'react'
import DeviceList from './components/DeviceList'
import Language from './components/Language'
import History from './components/History'
import Settings from './components/Settings'
import LanDevices from './components/LanDevices'
import { addRecord } from './services/history'
import { putRecvChunk, getRecvChunkIndices, assembleRecvFile, clearRecv, putRecvMeta, fileKey, getSendState, putSendState } from './services/db'
import { useSettings } from './store/settings'
import { generateKeyPair, exportPubJwk, importPubJwk, deriveAesGcmKey, aesEncrypt, aesDecrypt, makeIv } from './network/crypto'

function DeviceList({ ws, selfId, room, token, exp, sig, peers }: { ws: WebSocket | null, selfId: string, room: string, token: string, exp: number, sig: string, peers: { id: string, name?: string }[] }) {
  const [qr, setQr] = useState<string>('')
  useEffect(() => {
    const url = window.location.origin.replace('http:', 'https:')
    const qs = new URLSearchParams({ room, url, token, exp: String(exp), sig }).toString()
    fetch(`${url}/api/qr/room?${qs}`).then(r => r.json()).then(j => setQr(j.data))
  }, [room, token, exp, sig])
  return (
    <div>
      <div>房间 {room}</div>
      {qr && <img src={qr} alt="qr" style={{ width: 160, height: 160 }} />}
      <ul>
        <li>自己 {selfId}</li>
        {peers.map(p => <li key={p.id}>{p.name || p.id}</li>)}
      </ul>
      <button onClick={() => ws?.send(JSON.stringify({ t: 'peers' }))}>刷新设备</button>
    </div>
  )
}

function Transfer({ ws, selfId, peers }: { ws: WebSocket | null, selfId: string, peers: { id: string }[] }) {
  const [to, setTo] = useState<string>('')
  const [connected, setConnected] = useState<boolean>(false)
  const [logs, setLogs] = useState<string[]>([])
  const [dcReady, setDcReady] = useState<boolean>(false)
  const [textInput, setTextInput] = useState<string>('')
  const add = (s: string) => setLogs(l => [...l.slice(-50), s])
  const ctrlRef = React.useRef<RTCDataChannel | null>(null)
  const dataRef = React.useRef<RTCDataChannel | null>(null)
  const textRef = React.useRef<RTCDataChannel | null>(null)
  const pcRef = React.useRef<RTCPeerConnection | null>(null)
  const sendTimesRef = React.useRef<Map<number, number>>(new Map())
  const cwndRef = React.useRef<number>(16)
  const srttRef = React.useRef<number>(0)
  const rttvarRef = React.useRef<number>(0)
  const lossCountRef = React.useRef<number>(0)
  const sentCountRef = React.useRef<number>(0)
  const timeoutTimerRef = React.useRef<any>(null)
  const filesQueueRef = React.useRef<File[]>([])
  const lastSentRef = React.useRef<number>(0)
  const settings = useSettings()
  const myPrivRef = React.useRef<CryptoKey | null>(null)
  const myPubRef = React.useRef<CryptoKey | null>(null)
  const peerPubRef = React.useRef<CryptoKey | null>(null)
  const sessionKeyRef = React.useRef<CryptoKey | null>(null)
  const sendRef = React.useRef<any>(null)
  const recvRef = React.useRef<any>(null)
  const [eta, setEta] = useState<number>(0)
  const url = window.location.origin.replace('http:', 'https:')
  function setupPc(initiator: boolean) {
    const pc = new RTCPeerConnection()
    pc.onicecandidate = ev => { if (ev.candidate) ws?.send(JSON.stringify({ t: 'signal', to, data: { type: 'candidate', candidate: ev.candidate } })) }
    pc.onconnectionstatechange = () => { if (pc.connectionState === 'connected') setConnected(true) }
    const ctrl = pc.createDataChannel('ctrl')
    ctrlRef.current = ctrl
    ctrl.onopen = () => add('ctrl open')
    ctrl.onmessage = e => ctrlMessage(e)
    const data = pc.createDataChannel('data')
    data.binaryType = 'arraybuffer'
    dataRef.current = data
    data.onopen = () => { add('data open'); setDcReady(true) }
    data.onmessage = onData
    const text = pc.createDataChannel('text')
    textRef.current = text
    pc.ondatachannel = ev => {
      const ch = ev.channel
      if (ch.label === 'data') { ch.binaryType = 'arraybuffer'; ch.onopen = () => { add('data open'); setDcReady(true) }; ch.onmessage = onData }
      if (ch.label === 'ctrl') { ch.onopen = () => add('ctrl open'); ch.onmessage = ctrlMessage }
      if (ch.label === 'text') { ch.onopen = () => add('text open') }
    }
    pcRef.current = pc
    if (!timeoutTimerRef.current) timeoutTimerRef.current = setInterval(checkTimeouts, 1000)
    if (initiator) pc.createOffer().then(o => pc.setLocalDescription(o).then(() => ws?.send(JSON.stringify({ t: 'signal', to, data: o }))))
  }
  function ctrlMessage(e: MessageEvent) {
    const msg = JSON.parse(String(e.data))
    if (msg.type === 'meta') { recvRef.current = { id: msg.id, name: msg.name, size: msg.size, chunkSize: msg.chunkSize, total: msg.total, receivedBytes: 0, chunks: new Map(), receivedSet: new Set(), salt: msg.salt }; putRecvMeta(msg.id, { name: msg.name, size: msg.size, chunkSize: msg.chunkSize, total: msg.total, salt: msg.salt }); getRecvChunkIndices(msg.id).then(list => ctrlRef.current?.send(JSON.stringify({ type: 'state', id: msg.id, received: list }))) }
    if (msg.type === 'chunk') { }
    if (msg.type === 'ack') { const s = sendRef.current; if (s && s.id === msg.id && s.pending.has(msg.index)) { s.pending.delete(msg.index); s.ackedBytes += msg.bytes; const dt = (Date.now() - s.startedAt) / 1000; s.rateKBps = dt > 0 ? Math.round((s.ackedBytes / 1024) / dt) : 0; const bps = s.rateKBps * 1024; setEta(bps > 0 ? Math.ceil((s.size - s.ackedBytes) / bps) : 0); const st = sendTimesRef.current.get(msg.index); if (st) { const rtt = Date.now() - st; sentCountRef.current += 1; if (srttRef.current === 0) srttRef.current = rtt; else srttRef.current = 0.875 * srttRef.current + 0.125 * rtt; rttvarRef.current = 0.75 * rttvarRef.current + 0.25 * Math.abs(srttRef.current - rtt); const expected = (cwndRef.current * (s.chunkSize)) / (srttRef.current || 1); if (bps < expected * 0.8 && cwndRef.current > 4) cwndRef.current = Math.max(cwndRef.current - 1, 4); else if (bps > expected * 0.9 && cwndRef.current < 128) cwndRef.current = cwndRef.current + 1; sendTimesRef.current.delete(msg.index) } persistSendState(); if (s.ackedBytes >= s.size) { addRecord({ id: crypto.randomUUID(), name: s.name, size: s.size, peer: to, time: Date.now(), dir: 'out' }); const keystr = fileKey(s.name, s.size, (s.file as any).lastModified || 0); clearSendState(keystr); sendRef.current = null; dequeueNext() } else { scheduleSend() } } }
    if (msg.type === 'state') { const s = sendRef.current; if (s && s.id === msg.id) { const set = new Set<number>(msg.received || []); s.nextIndex = 0; s.pending.clear(); for (let i = 0; i < s.total; i++) { if (!set.has(i)) s.missing.add(i) } scheduleSend() } }
  }
  async function onData(e: MessageEvent) {
    const r = recvRef.current
    const key = sessionKeyRef.current
    if (!r || !key) return
    const buf = e.data as ArrayBuffer
    const idx = r.chunks.size
    const iv = makeIv(r.salt, idx)
    try {
      const plain = await aesDecrypt(key, iv, buf)
      r.chunks.set(idx, plain)
      r.receivedBytes += plain.byteLength
      r.receivedSet.add(idx)
      putRecvChunk(r.id, idx, plain as ArrayBuffer)
      ctrlRef.current?.send(JSON.stringify({ type: 'ack', id: r.id, index: idx, bytes: (plain as ArrayBuffer).byteLength }))
      if (r.receivedBytes >= r.size) finalizeReceive()
    } catch {}
  }
  async function finalizeReceive() {
    const r = recvRef.current
    if (!r) return
    const blob = await assembleRecvFile(r.id, r.total)
    const link = URL.createObjectURL(blob)
    addRecord({ id: crypto.randomUUID(), name: r.name, size: r.size, peer: to, time: Date.now(), dir: 'in' })
    await clearRecv(r.id)
  }
  useEffect(() => {
    if (!ws) return
    ws.onmessage = e => {
      const m = JSON.parse(e.data)
      if (m.t === 'signal') {
        const pc = pcRef.current
        if (!pc) return
        if (m.data.type === 'offer') pc.setRemoteDescription(m.data).then(() => pc.createAnswer().then(a => pc.setLocalDescription(a).then(() => ws.send(JSON.stringify({ t: 'signal', to: m.from, data: a })))) )
        else if (m.data.type === 'answer') pc.setRemoteDescription(m.data)
        else if (m.data.type === 'candidate') pc.addIceCandidate(m.data)
        else if (m.data.type === 'key') { importPubJwk(m.data.jwk).then(pub => { peerPubRef.current = pub; const priv = myPrivRef.current; if (priv) deriveAesGcmKey(priv, pub).then(k => { sessionKeyRef.current = k }) }) }
      }
    }
  }, [ws])
  async function connectPeer(id: string) { setTo(id); setupPc(true); const kp = await generateKeyPair(); myPrivRef.current = kp.privateKey; myPubRef.current = kp.publicKey; const jwk = await exportPubJwk(kp.publicKey); ws?.send(JSON.stringify({ t: 'signal', to: id, data: { type: 'key', jwk } })) }
  async function sendFileInput(f: File) {
    const ctrl = ctrlRef.current
    const data = dataRef.current
    if (!ctrl || !data || ctrl.readyState !== 'open' || data.readyState !== 'open') return
    const chunkSize = 256 * 1024
    const total = Math.ceil(f.size / chunkSize)
    const id = crypto.randomUUID()
    const salt = Math.random().toString(36).slice(2, 10)
    const keystr = fileKey(f.name, f.size, (f as any).lastModified || 0)
    const prev = await getSendState(keystr)
    sendRef.current = { id: prev?.id || id, name: f.name, size: f.size, chunkSize, total, nextIndex: 0, pending: new Set<number>(), ackedBytes: 0, startedAt: Date.now(), rateKBps: 0, paused: false, missing: new Set<number>(), file: f, cwnd: 16, salt }
    if (prev && Array.isArray(prev.acked)) { const set = new Set<number>(prev.acked); for (let i = 0; i < total; i++) { if (!set.has(i)) sendRef.current.missing.add(i) } sendRef.current.ackedBytes = prev.acked.length * chunkSize }
    ctrl.send(JSON.stringify({ type: 'meta', id, name: f.name, size: f.size, chunkSize, total, salt }))
    scheduleSend()
  }
  function enqueueFiles(fs: FileList) {
    const arr: File[] = []
    for (let i = 0; i < fs.length; i++) arr.push(fs[i])
    filesQueueRef.current = filesQueueRef.current.concat(arr)
    if (!sendRef.current) dequeueNext()
  }
  function dequeueNext() {
    const q = filesQueueRef.current
    if (q.length === 0) return
    const f = q.shift()!
    filesQueueRef.current = q
    sendFileInput(f)
  }
  async function sendChunk(index: number) {
    const s = sendRef.current
    const file: File = s.file
    const start = index * s.chunkSize
    const end = Math.min(start + s.chunkSize, s.size)
    const buf = await file.slice(start, end).arrayBuffer()
    const key = sessionKeyRef.current
    const iv = makeIv(s.salt || '', index)
    const enc = key ? await aesEncrypt(key, iv, buf) : buf
    const now = Date.now()
    if (settings.bandwidthKBps > 0 && lastSentRef.current) {
      const kb = (enc as ArrayBuffer).byteLength / 1024
      const needMs = (kb / settings.bandwidthKBps) * 1000
      const delta = now - lastSentRef.current
      if (delta < needMs) await new Promise(r => setTimeout(r, needMs - delta))
    }
    ctrlRef.current?.send(JSON.stringify({ type: 'chunk', id: s.id, index, bytes: (enc as ArrayBuffer).byteLength }))
    dataRef.current?.send(enc as ArrayBuffer)
    s.pending.add(index)
    sendTimesRef.current.set(index, Date.now())
    lastSentRef.current = Date.now()
  }
  async function fallbackHttp() {
    const s = sendRef.current
    if (!s || !s.file) return
    const id = s.id
    const base = url
    await fetch(`${base}/api/files/upload/init`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id, name: s.name, size: s.size }) })
    for (let i = 0; i < s.total; i++) {
      const start = i * s.chunkSize
      const end = Math.min(start + s.chunkSize, s.size)
      const buf = await s.file.slice(start, end).arrayBuffer()
      const key = sessionKeyRef.current
      const iv = makeIv(s.salt || '', i)
      const enc = key ? await aesEncrypt(key, iv, buf) : buf
      const form = new FormData()
      form.append('bin', new Blob([enc]))
      await fetch(`${base}/api/files/upload/chunk?id=${id}&index=${i}`, { method: 'POST', body: form })
    }
    await fetch(`${base}/api/files/upload/finish`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id, name: s.name, total: s.total }) })
    ctrlRef.current?.send(JSON.stringify({ type: 'http', id, name: s.name, size: s.size, url: `${base}/api/files/download?name=${encodeURIComponent(s.name)}` }))
  }
  async function downloadAndDecrypt(link: string, total: number, chunkSize: number, salt: string) {
    const key = sessionKeyRef.current
    if (!key) return
    const parts: BlobPart[] = []
    for (let i = 0; i < total; i++) {
      const start = i * chunkSize
      const end = start + chunkSize - 1
      const r = await fetch(link, { headers: { Range: `bytes=${start}-${end}` } })
      const enc = await r.arrayBuffer()
      const iv = makeIv(salt, i)
      const plain = await aesDecrypt(key, iv, enc)
      parts.push(plain)
    }
    const blob = new Blob(parts)
    addRecord({ id: crypto.randomUUID(), name: link.split('=')[1] || 'file', size: blob.size, peer: to, time: Date.now(), dir: 'in' })
  }
  function scheduleSend() {
    const s = sendRef.current
    if (!s || s.paused) return
    const windowSize = cwndRef.current
    while (s.pending.size < windowSize && s.nextIndex < s.total) {
      const i = s.nextIndex++
      sendChunk(i)
    }
  }
  function checkTimeouts() {
    const s = sendRef.current
    if (!s) return
    const now = Date.now()
    for (const [idx, st] of Array.from(sendTimesRef.current.entries())) {
      if (now - st > (srttRef.current || 1000) * 2 + 500) {
        s.pending.delete(idx)
        s.missing.add(idx)
        sendTimesRef.current.delete(idx)
        cwndRef.current = Math.max(Math.floor(cwndRef.current / 2), 4)
        lossCountRef.current += 1
      }
    }
    if (lossCountRef.current > 0 && sentCountRef.current > 0) {
      const loss = lossCountRef.current / sentCountRef.current
      if (loss > 0.05) cwndRef.current = Math.max(Math.floor(cwndRef.current * 0.7), 4)
    }
    scheduleSend()
  }
  function persistSendState() {
    const s = sendRef.current
    if (!s) return
    const acked: number[] = []
    for (let i = 0; i < s.total; i++) { if (!s.missing.has(i) && i < s.nextIndex) acked.push(i) }
    const keystr = fileKey(s.name, s.size, (s.file as any).lastModified || 0)
    putSendState(keystr, { id: s.id, name: s.name, size: s.size, chunkSize: s.chunkSize, total: s.total, acked })
  }
  return (
    <div>
      <Language />
      <Settings />
      <LanDevices />
      <div>选择设备</div>
      <select value={to} onChange={e => connectPeer(e.target.value)}>
        <option value="">请选择</option>
        {peers.filter(p => p.id !== selfId).map(p => <option key={p.id} value={p.id}>{p.id}</option>)}
      </select>
      <div>{connected ? '已连接' : '未连接'}</div>
      <input type="file" multiple webkitdirectory="" onChange={e => { const fs = e.target.files; if (fs && fs.length) enqueueFiles(fs) }} />
      <div>
        <textarea rows={2} value={textInput} onChange={e => setTextInput(e.target.value)} placeholder="输入文本发送" />
        <button onClick={() => { const ch = textRef.current; if (ch && ch.readyState === 'open' && textInput.trim()) { ch.send(textInput); addRecord({ id: crypto.randomUUID(), name: '[text]', size: textInput.length, peer: to, time: Date.now(), dir: 'out' }); setTextInput('') } }}>发送文本</button>
      </div>
      <div>ETA {eta}s</div>
      <div>{logs.map((l, i) => <div key={i}>{l}</div>)}</div>
      <History />
    </div>
  )
}

export default function App() {
  const [room, setRoom] = useState<string>('')
  const [token, setToken] = useState<string>('')
  const [exp, setExp] = useState<number>(0)
  const [sig, setSig] = useState<string>('')
  const [ws, setWs] = useState<WebSocket | null>(null)
  const [id, setId] = useState<string>('')
  const [peers, setPeers] = useState<{ id: string, name?: string }[]>([])
  const [blocked, setBlocked] = useState<boolean>(false)
  const url = useMemo(() => window.location.origin.replace('http:', 'https:'), [])
  useEffect(() => {
    (async () => {
      const params = new URLSearchParams(window.location.search)
      const r = params.get('room') || ''
      const v = r || Math.random().toString(36).slice(2, 8)
      setRoom(v)
      const cert = await fetch(`${url}/api/cert`).then(r => r.json())
      const qfp = params.get('fp') || ''
      if (qfp && qfp !== cert.fp) { setBlocked(true); return }
      let tkv = params.get('token') || ''
      let expv = Number(params.get('exp') || 0)
      let sigv = params.get('sig') || ''
      if (!tkv || !expv || !sigv) {
        const j = await fetch(`${url}/api/room/token?room=${v}`).then(r => r.json())
        tkv = j.token; expv = j.exp; sigv = j.sig
      }
      setToken(tkv); setExp(expv); setSig(sigv)
      const s = new WebSocket(`${url.replace('https', 'wss')}`)
      s.onmessage = e => {
        const m = JSON.parse(e.data)
        if (m.t === 'hello') setId(m.id)
        if (m.t === 'peers') setPeers(m.peers)
        if (m.t === 'leave') setPeers(p => p.filter(x => x.id !== m.id))
        if (m.t === 'signal') {}
      }
      s.onopen = () => { s.send(JSON.stringify({ t: 'join', room: v, token: tkv, exp: expv, sig: sigv })); s.send(JSON.stringify({ t: 'peers' })) }
      setWs(s)
    })()
    return () => ws?.close()
  }, [url])
  if (blocked) return <div>指纹不匹配，已阻止连接</div>
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, padding: 16 }}>
      <div>
        <h2>设备</h2>
        <DeviceList ws={ws} selfId={id} room={room} token={token} exp={exp} sig={sig} peers={peers} />
        <h2>文件</h2>
        <div>文件管理接口待接入</div>
      </div>
      <div>
        <h2>传输</h2>
        <Transfer ws={ws} selfId={id} peers={peers} />
        <h2>历史</h2>
        <div>历史记录待接入</div>
      </div>
    </div>
  )
}
