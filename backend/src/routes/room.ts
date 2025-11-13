import express from 'express'
import { createRoomSigned } from '../security/rooms.js'

const router = express.Router()

router.get('/create', (req, res) => {
  const room = String(req.query.room || '')
  if (!room) return res.status(400).json({ error: 'room' })
  const s = createRoomSigned(room)
  res.json({ room, token: s.token, exp: s.exp, sig: s.sig })
})

router.get('/token', (req, res) => {
  const room = String(req.query.room || '')
  if (!room) return res.status(400).json({ error: 'room' })
  const s = createRoomSigned(room)
  res.json({ room, token: s.token, exp: s.exp, sig: s.sig })
})

export default router