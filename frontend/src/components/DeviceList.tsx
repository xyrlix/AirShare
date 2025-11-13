import React, { useEffect, useState } from 'react'

export function DeviceList({ ws, selfId, room, token, exp, sig, peers }: { ws: WebSocket | null, selfId: string, room: string, token: string, exp: number, sig: string, peers: { id: string, name?: string }[] }) {
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