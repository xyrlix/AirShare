import React, { useEffect, useMemo, useState } from 'react'
import { DeviceList } from './components/DeviceList'
import { FileManager } from './components/FileManager'
import { Transfer } from './components/Transfer'
import { useTranslation } from 'react-i18next'
import { Language } from './components/Language'
import { History } from './components/History'

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
      }
      s.onopen = () => {
        s.send(JSON.stringify({ t: 'join', room: v, token: tkv, exp: expv, sig: sigv }))
        s.send(JSON.stringify({ t: 'peers' }))
      }
      setWs(s)
    })()
    return () => ws?.close()
  }, [url])
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, padding: 16 }}>
      <Language />
      <div>
        <h2>{useTranslation().t('devices')}</h2>
        <DeviceList ws={ws} selfId={id} room={room} token={token} exp={exp} sig={sig} peers={peers} />
        <h2>{useTranslation().t('files')}</h2>
        <FileManager base={`${url}/api/files`} />
      </div>
      <div>
        <h2>{useTranslation().t('transfer')}</h2>
        <Transfer ws={ws} selfId={id} peers={peers} />
        <History />
      </div>
    </div>
  )
}