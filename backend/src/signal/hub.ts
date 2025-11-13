import { WebSocketServer, WebSocket } from 'ws'
import { v4 as uuidv4 } from 'uuid'
import { verifySigned } from '../security/rooms.js'

type Client = { id: string, ws: WebSocket, room?: string, name?: string, token?: string, exp?: number, sig?: string }

export function createHub(wss: WebSocketServer) {
  const clients = new Map<string, Client>()
  wss.on('connection', ws => {
    const id = uuidv4()
    const client: Client = { id, ws }
    clients.set(id, client)
    ws.send(JSON.stringify({ t: 'hello', id }))
    ws.on('message', d => {
      try {
        const m = JSON.parse(d.toString())
        if (m.t === 'join') {
          client.room = m.room
          client.token = m.token
          client.exp = m.exp
          client.sig = m.sig
          const ok = client.room && client.token && client.exp && client.sig && verifySigned(String(client.room), String(client.token), Number(client.exp), String(client.sig))
          if (!ok) { try { ws.close() } catch {} }
        }
        if (m.t === 'name') client.name = m.name
        if (m.t === 'signal') {
          const to = clients.get(m.to)
          const ok = client.room && client.token && client.exp && client.sig && verifySigned(String(client.room), String(client.token), Number(client.exp), String(client.sig))
          if (to && to.room === client.room && ok) {
            to.ws.send(JSON.stringify({ t: 'signal', from: id, data: m.data }))
          }
        }
        if (m.t === 'peers') {
          const peers = [...clients.values()].filter(p => p.room === client.room && p.id !== id).map(p => ({ id: p.id, name: p.name }))
          ws.send(JSON.stringify({ t: 'peers', peers }))
        }
      } catch {}
    })
    ws.on('close', () => {
      clients.delete(id)
      const payload = JSON.stringify({ t: 'leave', id })
      for (const p of clients.values()) if (p.room === client.room) p.ws.send(payload)
    })
  })
  return { clients }
}