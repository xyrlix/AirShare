import fs from 'fs'
import path from 'path'
import https from 'https'
import express from 'express'
import cors from 'cors'
import crypto from 'crypto'
import selfsigned from 'selfsigned'
import filesRouter from './api/files.js'
import qrRouter from './api/qr.js'
import sessionRouter from './api/session.js'
import { createRoomSigned } from './security/token.js'
import { createHub } from './signaling/hub.js'

const app = express()
const allowed = (process.env.ALLOWED_ORIGINS || '').split(',').map(s => s.trim()).filter(Boolean)
app.use(cors({ origin: (origin, cb) => { if (!origin) return cb(null, true); if (allowed.length === 0 || allowed.includes(origin)) return cb(null, true); cb(new Error('origin')) } }))
app.use(express.json())
app.use('/api/files', filesRouter)
app.use('/api/qr', qrRouter)
app.use('/api/session', sessionRouter)
app.use('/api/room', roomRouter)

const dataDir = path.join(process.cwd(), 'backend', 'data')
fs.mkdirSync(dataDir, { recursive: true })

const attrs = [{ name: 'commonName', value: 'AirShare' }]
const pems = selfsigned.generate(attrs, { days: 365, keySize: 2048 })
const server = https.createServer({ key: pems.private, cert: pems.cert }, app)
const fingerprint = crypto.createHash('sha256').update(pems.cert).digest('hex')
app.locals.fp = fingerprint

createHub(server)

app.get('/api/room/token', (req, res) => {
  const room = String(req.query.room || '')
  if (!room) return res.status(400).json({ error: 'room' })
  const s = createRoomSigned(room)
  res.json({ room, token: s.token, exp: s.exp, sig: s.sig })
})

app.get('/api/cert', (_, res) => res.json({ fp: fingerprint }))

const publicDir = path.join(process.cwd(), 'frontend', 'dist')
if (fs.existsSync(publicDir)) app.use('/', express.static(publicDir))

const port = Number(process.env.PORT || 8443)
server.listen(port)
console.log(`AirShare https://localhost:${port}`)
