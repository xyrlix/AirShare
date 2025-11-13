import express from 'express'
import QRCode from 'qrcode'
import { createRoomSigned } from '../security/token.js'

const router = express.Router()
router.get('/room', async (req, res) => {
  const room = String(req.query.room || '')
  const url = String(req.query.url || '')
  if (!room || !url) return res.status(400).json({ error: 'param' })
  const tok = String(req.query.token || '')
  const expq = Number(req.query.exp || 0)
  const sigq = String(req.query.sig || '')
  const s = tok && expq && sigq ? { token: tok, exp: expq, sig: sigq } : createRoomSigned(room)
  const fp = String(req.app.locals.fp || '')
  const text = `${url}?room=${room}&token=${s.token}&exp=${s.exp}&sig=${s.sig}${fp ? `&fp=${fp}` : ''}`
  const data = await QRCode.toDataURL(text)
  res.json({ data, room, token: s.token, exp: s.exp, sig: s.sig, fp })
})

export default router
