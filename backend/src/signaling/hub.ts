import { WebSocketServer, WebSocket } from 'ws'
import crypto from 'crypto'
import { verifySigned } from '../security/token.js'

export type Client = { id: string, ws: WebSocket, room?: string, token?: string, exp?: number, sig?: string, name?: string }

export function createHub(server: any) {
  const wss = new WebSocketServer({ server })
  const clients = new Map<string, Client>()
  wss.on('connection', ws => {
    const id = crypto.randomUUID()
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
        } else if (m.t === 'peers') {
          const list = Array.from(clients.values()).filter(c => c.room === client.room).map(c => ({ id: c.id, name: c.name }))
          ws.send(JSON.stringify({ t: 'peers', peers: list }))
        } else if (m.t === 'signal') {
          const to = clients.get(m.to)
          const ok = client.room && client.token && client.exp && client.sig && verifySigned(String(client.room), String(client.token), Number(client.exp), String(client.sig))
          if (to && to.room === client.room && ok) to.ws.send(JSON.stringify({ t: 'signal', from: id, data: m.data }))
        }
      } catch {}
    })
    ws.on('close', () => { clients.delete(id); const list = Array.from(clients.values()).filter(c => c.room === client.room); list.forEach(c => c.ws.send(JSON.stringify({ t: 'leave', id }))) })
  })
  return wss
}
