import express from 'express'

const router = express.Router()
router.post('/key', (req, res) => {
  const { room, jwk, version } = req.body || {}
  if (!room || !jwk) return res.status(400).json({ error: 'param' })
  const v = Number(version || 1)
  res.json({ ok: true, version: v })
})

router.get('/info', (_req, res) => {
  res.json({ version: 1, ecdh: 'P-256', cipher: 'AES-GCM', iv: 'salt:index derived 12 bytes' })
})

export default router
