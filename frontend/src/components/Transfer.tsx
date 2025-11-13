import React, { useEffect, useRef, useState } from 'react'
import { addRecord } from '@/services/history'
import { putRecvChunk, getRecvChunkIndices, assembleRecvFile, clearRecv, putRecvMeta, fileKey, putSendAckForFile, getSendAckForFile, clearSendForFile } from '@/services/db'

type SendState = {
  id: string
  name: string
  size: number
  chunkSize: number
  total: number
  nextIndex: number
  pending: Set<number>
  ackedBytes: number
  startedAt: number
  rateKBps: number
  paused: boolean
  missing: Set<number>
  file?: File
  cwnd: number
  sendTimes: Map<number, number>
  srtt: number
  rttvar: number
  lossCount: number
  sentCount: number
}

type RecvState = {
  id: string
  name: string
  size: number
  chunkSize: number
  total: number
  receivedBytes: number
  chunks: Map<number, ArrayBuffer>
  expectQueue: number[]
  link?: string
  receivedSet: Set<number>
}

export function Transfer({ ws, selfId, peers }: { ws: WebSocket | null, selfId: string, peers: { id: string, name?: string }[] }) {
  const [to, setTo] = useState<string>('')
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const ctrlRef = useRef<RTCDataChannel | null>(null)
  const dataRef = useRef<RTCDataChannel | null>(null)
  const [log, setLog] = useState<string[]>([])
  const sendRef = useRef<SendState | null>(null)
  const recvRef = useRef<RecvState | null>(null)
  const samplesRef = useRef<{ t: number, bytes: number }[]>([])
  const [eta, setEta] = useState<number>(0)
  const [dcReady, setDcReady] = useState<boolean>(false)
  const url = window.location.origin.replace('http:', 'https:')
  const [dir, setDir] = useState<any>(null)
  const timerRef = useRef<any>(null)
  const badIceRef = useRef<number>(0)

  useEffect(() => {
    if (!ws) return
    const handler = (e: MessageEvent) => {
      const m = JSON.parse((e as MessageEvent).data)
      if (m.t === 'signal' && m.from === to) handleSignal(m.from, m.data)
    }
    ws.addEventListener('message', handler)
    return () => ws.removeEventListener('message', handler)
  }, [ws, to])

  function add(s: string) { setLog(l => [...l, s].slice(-100)) }

  function setupPc(initiator: boolean) {
    const pc = new RTCPeerConnection()
    pc.onicecandidate = ev => { if (ev.candidate) ws?.send(JSON.stringify({ t: 'signal', to, data: { type: 'candidate', candidate: ev.candidate } })) }
    pc.onconnectionstatechange = () => add(`pc ${pc.connectionState}`)
    pc.onconnectionstatechange = () => { add(`pc ${pc.connectionState}`); if (pc.connectionState === 'disconnected' || pc.connectionState === 'failed') attemptResume() }
    pc.oniceconnectionstatechange = () => {
      const st = pc.iceConnectionState
      if (st === 'failed' || st === 'disconnected') { if (!badIceRef.current) badIceRef.current = Date.now() }
      else { badIceRef.current = 0 }
      if (badIceRef.current && Date.now() - badIceRef.current > 3000) fallbackHttp()
    }
    pcRef.current = pc

    if (initiator) {
      const ctrl = pc.createDataChannel('ctrl')
      const data = pc.createDataChannel('data')
      ctrl.binaryType = 'arraybuffer'
      data.binaryType = 'arraybuffer'
      ctrlRef.current = ctrl
      dataRef.current = data
      bindCtrl(ctrl)
      bindData(data)
    } else {
      pc.ondatachannel = ev => {
        const ch = ev.channel
        if (ch.label === 'ctrl') { ctrlRef.current = ch; ch.binaryType = 'arraybuffer'; bindCtrl(ch) }
        if (ch.label === 'data') { dataRef.current = ch; ch.binaryType = 'arraybuffer'; bindData(ch) }
      }
    }
  }

  async function connect() {
    setupPc(true)
    const pc = pcRef.current!
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    ws?.send(JSON.stringify({ t: 'signal', to, data: offer }))
  }

  async function handleSignal(from: string, data: any) {
    const pc = pcRef.current || (setupPc(false), pcRef.current!)
    if (data.type === 'offer') {
      await pc.setRemoteDescription(data)
      const answer = await pc.createAnswer()
      await pc.setLocalDescription(answer)
      ws?.send(JSON.stringify({ t: 'signal', to: from, data: answer }))
    } else if (data.type === 'answer') {
      await pc.setRemoteDescription(data)
    } else if (data.type === 'candidate') {
      await pc.addIceCandidate(data.candidate)
    }
  }

  function bindCtrl(ctrl: RTCDataChannel) {
    ctrl.onopen = () => add('ctrl open')
    ctrl.onmessage = e => {
      if (typeof e.data === 'string') {
        const msg = JSON.parse(e.data)
        if (msg.type === 'meta') {
          recvRef.current = { id: msg.id, name: msg.name, size: msg.size, chunkSize: msg.chunkSize, total: msg.total, receivedBytes: 0, chunks: new Map(), expectQueue: [], receivedSet: new Set() }
          putRecvMeta(msg.id, { name: msg.name, size: msg.size, chunkSize: msg.chunkSize, total: msg.total })
          getRecvChunkIndices(msg.id).then(list => ctrl.send(JSON.stringify({ type: 'state', id: msg.id, received: list })))
          add(`meta ${msg.name} ${Math.round(msg.size / 1024)}KB`)
        } else if (msg.type === 'chunk') {
          const r = recvRef.current
          if (!r || r.id !== msg.id) return
          r.expectQueue.push(msg.index)
        } else if (msg.type === 'ack') {
          const s = sendRef.current
          if (!s || s.id !== msg.id) return
          if (s.pending.has(msg.index)) {
            s.pending.delete(msg.index)
            s.ackedBytes += msg.bytes
            const dt = (Date.now() - s.startedAt) / 1000
            s.rateKBps = dt > 0 ? Math.round((s.ackedBytes / 1024) / dt) : 0
            samplesRef.current.push({ t: Date.now(), bytes: s.ackedBytes })
            samplesRef.current = samplesRef.current.slice(-200)
            const last = samplesRef.current[samplesRef.current.length - 1]
            const first = samplesRef.current[0]
            const bps = last && first ? ((last.bytes - first.bytes) / ((last.t - first.t) / 1000)) : 0
            setEta(bps > 0 ? Math.ceil((s.size - s.ackedBytes) / bps) : 0)
            const st = s.sendTimes.get(msg.index)
            if (st) {
              const rtt = Date.now() - st
              if (rtt < 2000 && s.cwnd < 128) s.cwnd += 1
              s.sendTimes.delete(msg.index)
              s.sentCount += 1
              if (s.srtt === 0) s.srtt = rtt
              else s.srtt = 0.875 * s.srtt + 0.125 * rtt
              s.rttvar = 0.75 * s.rttvar + 0.25 * Math.abs(s.srtt - rtt)
              const expected = (s.cwnd * s.chunkSize) / (s.srtt || 1)
              const actual = bps
              if (actual < expected * 0.8 && s.cwnd > 4) s.cwnd = Math.max(s.cwnd - 1, 4)
              else if (actual > expected * 0.9 && s.cwnd < 128) s.cwnd += 1
            }
            persistAck(msg.index)
            if (s.ackedBytes >= s.size) { const key = fileKey(s.name, s.size, (s.file as any)?.lastModified || 0); clearSendForFile(key) }
            scheduleSend()
          }
        } else if (msg.type === 'ready') {
          scheduleSend()
        } else if (msg.type === 'state') {
          const s = sendRef.current
          if (!s || s.id !== msg.id) return
          s.missing = new Set()
          for (let i = 0; i < s.total; i++) if (!msg.received.includes(i)) s.missing.add(i)
          s.nextIndex = 0
          s.pending.clear()
          scheduleSend()
        } else if (msg.type === 'resume') {
          const r = recvRef.current
          if (!r || r.id !== msg.id) return
          ctrl.send(JSON.stringify({ type: 'state', id: r.id, received: Array.from(r.receivedSet.values()) }))
        } else if (msg.type === 'http') {
          const r = recvRef.current
          if (!r || r.id !== msg.id) return
          r.link = msg.url
          addRecord({ id: crypto.randomUUID(), name: msg.name, size: msg.size, peer: to, time: Date.now(), dir: 'in' })
          add('receive http link')
        }
      }
    }
  }

  function bindData(data: RTCDataChannel) {
    data.onopen = () => { add('data open'); setDcReady(true); if (!timerRef.current) timerRef.current = setInterval(checkTimeouts, 1000) }
    data.onmessage = e => {
      const r = recvRef.current
      if (!r) return
      const idx = r.expectQueue.shift()
      if (idx === undefined) return
    const buf = e.data as ArrayBuffer
    r.chunks.set(idx, buf)
    r.receivedBytes += buf.byteLength
    r.receivedSet.add(idx)
    putRecvChunk(r.id, idx, buf)
    ctrlRef.current?.send(JSON.stringify({ type: 'ack', id: r.id, index: idx, bytes: buf.byteLength }))
    if (r.receivedBytes >= r.size) finalizeReceive()
  }
  }

  async function finalizeReceive() {
    const r = recvRef.current
    if (!r) return
    const blob = await assembleRecvFile(r.id, r.total)
    const url = URL.createObjectURL(blob)
    r.link = url
    if (dir) {
      try {
        const path = r.name.split('/')
        let d = dir
        for (let i = 0; i < path.length - 1; i++) d = await d.getDirectoryHandle(path[i], { create: true })
        const fh = await d.getFileHandle(path[path.length - 1], { create: true })
        const w = await fh.createWritable()
        await w.write(blob)
        await w.close()
      } catch {}
    }
    addRecord({ id: crypto.randomUUID(), name: r.name, size: r.size, peer: to, time: Date.now(), dir: 'in' })
    add('receive done')
    await clearRecv(r.id)
  }

  async function sendFile(f: File) {
    const ctrl = ctrlRef.current
    const data = dataRef.current
    if (!ctrl || !data || ctrl.readyState !== 'open' || data.readyState !== 'open') return
    const chunkSize = 256 * 1024
    const total = Math.ceil(f.size / chunkSize)
    const key = fileKey(f.webkitRelativePath || f.name, f.size, (f as any).lastModified || 0)
    const prev = await getSendAckForFile(key)
    const id = prev?.id || crypto.randomUUID()
    sendRef.current = { id, name: f.webkitRelativePath || f.name, size: f.size, chunkSize, total, nextIndex: 0, pending: new Set(), ackedBytes: 0, startedAt: Date.now(), rateKBps: 0, paused: false, missing: new Set(), file: f, cwnd: 16, sendTimes: new Map(), srtt: 0, rttvar: 0, lossCount: 0, sentCount: 0 }
    if (prev && Array.isArray(prev.acked)) {
      for (const i of prev.acked) sendRef.current.missing.delete(i)
      const ackedBytes = prev.acked.length * chunkSize
      sendRef.current.ackedBytes = Math.min(ackedBytes, f.size)
    }
    ctrl.send(JSON.stringify({ type: 'meta', id, name: f.webkitRelativePath || f.name, size: f.size, chunkSize, total }))
    scheduleSend()
  }

  async function sendChunk(index: number) {
    const s = sendRef.current
    const data = dataRef.current
    const ctrl = ctrlRef.current
    if (!s || !data || !ctrl) return
    const start = index * s.chunkSize
    const end = Math.min(s.size, start + s.chunkSize)
    const file = s.file || (document.querySelector('#transfer-file') as HTMLInputElement)?.files?.[0]
    if (!file) return
    const buf = await file.slice(start, end).arrayBuffer()
    ctrl.send(JSON.stringify({ type: 'chunk', id: s.id, index, bytes: buf.byteLength }))
    data.send(buf)
    s.pending.add(index)
    s.sendTimes.set(index, Date.now())
  }

  function scheduleSend() {
    const s = sendRef.current
    if (!s || s.paused) return
    const windowSize = s.cwnd
    if (s.missing.size > 0) {
      const iter = Array.from(s.missing.values())
      while (s.pending.size < windowSize && iter.length > 0) {
        const i = iter.shift()!
        s.missing.delete(i)
        sendChunk(i)
      }
    } else {
      while (s.pending.size < windowSize && s.nextIndex < s.total) {
        const i = s.nextIndex++
        sendChunk(i)
      }
    }
  }

  function pause() { const s = sendRef.current; if (s) s.paused = true }
  function resume() { const s = sendRef.current; if (s) { s.paused = false; scheduleSend() } }

  function attemptResume() {
    const ctrl = ctrlRef.current
    const s = sendRef.current
    if (ctrl && s) ctrl.send(JSON.stringify({ type: 'resume', id: s.id }))
    setTimeout(() => { if (!dcReady) fallbackHttp() }, 5000)
  }

  async function fallbackHttp() {
    const s = sendRef.current
    if (!s || !s.file) return
    const id = s.id
    await fetch(`${url}/api/files/upload/init`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id, name: s.name, size: s.size }) })
    let i = 0
    while (i < s.size) {
      const buf = await s.file.slice(i, Math.min(i + s.chunkSize, s.size)).arrayBuffer()
      await fetch(`${url}/api/files/upload/chunk?id=${id}&index=${Math.floor(i / s.chunkSize)}`, { method: 'POST', body: buf })
      i += s.chunkSize
    }
    await fetch(`${url}/api/files/upload/finish`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id }) })
    ctrlRef.current?.send(JSON.stringify({ type: 'http', id, name: s.name, size: s.size, url: `${url}/api/files/download?name=${encodeURIComponent(s.name)}` }))
    addRecord({ id: crypto.randomUUID(), name: s.name, size: s.size, peer: to, time: Date.now(), dir: 'out' })
  }

  function checkTimeouts() {
    const s = sendRef.current
    if (!s) return
    const now = Date.now()
    const rtt = 1000
    for (const idx of Array.from(s.pending.values())) {
      const st = s.sendTimes.get(idx)
      if (st && now - st > rtt * 2 + 500) {
        s.pending.delete(idx)
        s.missing.add(idx)
        s.sendTimes.delete(idx)
        s.cwnd = Math.max(Math.floor(s.cwnd / 2), 4)
        s.lossCount += 1
      }
    }
    if (s.lossCount > 0 && s.sentCount > 0) {
      const loss = s.lossCount / s.sentCount
      if (loss > 0.05) s.cwnd = Math.max(Math.floor(s.cwnd * 0.7), 4)
    }
    scheduleSend()
  }

  function persistAck(index: number) {
    const s = sendRef.current
    if (!s || !s.file) return
    const key = fileKey(s.name, s.size, (s.file as any).lastModified || 0)
    const acked = s.pending.size > 0 ? [] : []
    const array = [] as number[]
    for (let i = 0; i < Math.ceil(s.size / s.chunkSize); i++) {
      if (!s.missing.has(i) && i < s.nextIndex) array.push(i)
    }
    putSendAckForFile(key, { id: s.id, chunkSize: s.chunkSize, total: s.total, acked: array, name: s.name, size: s.size })
  }

  const sendPercent = (() => { const s = sendRef.current; return s ? Math.floor((s.ackedBytes / s.size) * 100) : 0 })()
  const recvPercent = (() => { const r = recvRef.current; return r ? Math.floor((r.receivedBytes / r.size) * 100) : 0 })()

  return (
    <div>
      <select value={to} onChange={e => setTo(e.target.value)}>
        <option value="">选择设备</option>
        {peers.map(p => <option key={p.id} value={p.id}>{p.name || p.id}</option>)}
      </select>
      <button onClick={connect} disabled={!to}>连接</button>
      <input id="transfer-file" type="file" multiple webkitdirectory="" onChange={e => { const files = e.target.files; if (files) { for (let i = 0; i < files.length; i++) sendFile(files[i]) } }} />
      <div>
        <div>发送进度 {sendPercent}% 速率 {sendRef.current?.rateKBps || 0} KB/s 剩余 {eta}s 缺片 {sendRef.current?.missing.size || 0}</div>
        <button onClick={pause}>暂停</button>
        <button onClick={resume}>继续</button>
        <button onClick={async () => { try { const d: any = await (window as any).showDirectoryPicker(); setDir(d) } catch {} }}>选择目录</button>
      </div>
      <div>
        <div>接收进度 {recvPercent}% {recvRef.current?.link && <a href={recvRef.current.link} download={recvRef.current.name}>保存</a>}</div>
      </div>
      <div>{log.map((s, i) => <div key={i}>{s}</div>)}</div>
    </div>
  )
}