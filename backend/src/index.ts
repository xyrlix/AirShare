import fs from 'fs'
import path from 'path'
import https from 'https'
import express from 'express'
import cors from 'cors'
import crypto from 'crypto'
import { v4 as uuidv4 } from 'uuid'
import selfsigned from 'selfsigned'
import { WebSocketServer } from 'ws'
import { createMdns } from './net/mdns.js'
import { createUdpDiscovery } from './net/udp.js'
import { createHub } from './signal/hub.js'
import filesRouter from './routes/files.js'
import qrRouter from './routes/qr.js'
import roomRouter from './routes/room.js'

const app = express()
const allowed = (process.env.ALLOWED_ORIGINS || '').split(',').map(s => s.trim()).filter(Boolean)
app.use(cors({ origin: (origin, cb) => { if (!origin) return cb(null, true); if (allowed.length === 0 || allowed.includes(origin)) return cb(null, true); cb(new Error('origin')) } }))
app.use(express.json())
app.use('/api/files', filesRouter)
app.use('/api/qr', qrRouter)
app.use('/api/room', roomRouter)

const dataDir = path.join(process.cwd(), 'backend', 'data')
fs.mkdirSync(dataDir, { recursive: true })

const attrs = [{ name: 'commonName', value: 'AirShare' }]
const pems = selfsigned.generate(attrs, { days: 365, keySize: 2048 })
const server = https.createServer({ key: pems.private, cert: pems.cert }, app)
const fingerprint = crypto.createHash('sha256').update(pems.cert).digest('hex')
app.locals.fp = fingerprint

const wss = new WebSocketServer({ server })
const hub = createHub(wss)

createMdns(8443)
createUdpDiscovery(8443)

app.get('/api/ping', (_, res) => res.json({ id: uuidv4(), ok: true }))
app.get('/api/cert', (_, res) => res.json({ fp: fingerprint }))

const publicDir = path.join(process.cwd(), 'frontend', 'dist')
if (fs.existsSync(publicDir)) app.use('/', express.static(publicDir))

const port = Number(process.env.PORT || 8443)
server.listen(port)
console.log(`AirShare https://localhost:${port}`)