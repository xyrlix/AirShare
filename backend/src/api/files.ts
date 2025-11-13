import express from 'express'
import fs from 'fs'
import path from 'path'
import multer from 'multer'

const router = express.Router()
const mem = multer({ storage: multer.memoryStorage() })
const dataDir = path.join(process.cwd(), 'backend', 'data')
fs.mkdirSync(dataDir, { recursive: true })
const chunks = new Map<string, Map<number, Buffer>>()

router.get('/list', (_req, res) => {
  const list = fs.readdirSync(dataDir).map(name => ({ name, size: fs.statSync(path.join(dataDir, name)).size }))
  res.json({ files: list })
})

router.post('/upload/init', (req, res) => {
  const { id } = req.body || {}
  if (!id) return res.status(400).json({ error: 'param' })
  chunks.set(id, new Map())
  res.json({ ok: true })
})

router.post('/upload/chunk', mem.single('bin'), (req, res) => {
  const id = String(req.query.id || '')
  const index = Number(req.query.index || 0)
  if (!id || !req.file) return res.status(400).json({ error: 'param' })
  const m = chunks.get(id) || new Map<number, Buffer>()
  m.set(index, req.file.buffer)
  chunks.set(id, m)
  res.json({ ok: true })
})

router.post('/upload/finish', (req, res) => {
  const { id, name, total } = req.body || {}
  const m = chunks.get(id)
  if (!m || !name || !total) return res.status(400).json({ error: 'param' })
  const list: Buffer[] = []
  for (let i = 0; i < total; i++) {
    const b = m.get(i)
    if (b) list.push(b)
  }
  const buf = Buffer.concat(list)
  fs.writeFileSync(path.join(dataDir, name), buf)
  chunks.delete(id)
  res.json({ ok: true })
})

router.get('/download', (req, res) => {
  const name = String(req.query.name || '')
  const p = path.join(dataDir, name)
  if (!fs.existsSync(p)) return res.status(404).end()
  const stat = fs.statSync(p)
  const range = req.headers.range
  if (range) {
    const match = /bytes=(\d+)-(\d+)?/.exec(range)
    const start = match ? parseInt(match[1], 10) : 0
    const end = match && match[2] ? parseInt(match[2], 10) : stat.size - 1
    const chunkSize = end - start + 1
    res.status(206)
    res.setHeader('Content-Range', `bytes ${start}-${end}/${stat.size}`)
    res.setHeader('Accept-Ranges', 'bytes')
    res.setHeader('Content-Length', String(chunkSize))
    const stream = fs.createReadStream(p, { start, end })
    stream.pipe(res)
  } else {
    res.setHeader('Content-Length', String(stat.size))
    fs.createReadStream(p).pipe(res)
  }
})

router.delete('/delete', (req, res) => {
  const name = String(req.query.name || '')
  const p = path.join(dataDir, name)
  if (!fs.existsSync(p)) return res.status(404).end()
  fs.unlinkSync(p)
  res.json({ ok: true })
})

router.patch('/rename', (req, res) => {
  const { from, to } = req.body || {}
  if (!from || !to) return res.status(400).json({ error: 'param' })
  fs.renameSync(path.join(dataDir, from), path.join(dataDir, to))
  res.json({ ok: true })
})

export default router
