import express from 'express'

const router = express.Router()
router.post('/key', (req, res) => {
  const { room, jwk } = req.body || {}
  if (!room || !jwk) return res.status(400).json({ error: 'param' })
  res.json({ ok: true })
})

router.get('/info', (_req, res) => {
  res.json({ ecdh: 'P-256', cipher: 'AES-GCM', iv: 'salt:index derived 12 bytes' })
})

export default router
