import fs from 'fs'
import path from 'path'
import express from 'express'

const router = express.Router()
const root = path.join(process.cwd(), 'backend', 'data')

router.get('/', (_, res) => {
  const list = fs.readdirSync(root).map(n => ({ name: n, isDir: fs.statSync(path.join(root, n)).isDirectory() }))
  res.json(list)
})

router.post('/mkdir', (req, res) => {
  const name = String(req.body.name || '')
  if (!name) return res.status(400).json({ error: 'name' })
  fs.mkdirSync(path.join(root, name), { recursive: true })
  res.json({ ok: true })
})

router.post('/rename', (req, res) => {
  const from = String(req.body.from || '')
  const to = String(req.body.to || '')
  if (!from || !to) return res.status(400).json({ error: 'param' })
  fs.renameSync(path.join(root, from), path.join(root, to))
  res.json({ ok: true })
})

router.post('/delete', (req, res) => {
  const name = String(req.body.name || '')
  if (!name) return res.status(400).json({ error: 'name' })
  const p = path.join(root, name)
  const s = fs.statSync(p)
  if (s.isDirectory()) fs.rmSync(p, { recursive: true, force: true })
  else fs.unlinkSync(p)
  res.json({ ok: true })
})

router.get('/download', (req, res) => {
  const name = String(req.query.name || '')
  const p = path.join(root, name)
  if (!fs.existsSync(p)) return res.status(404).end()
  res.download(p)
})

router.post('/upload/init', (req, res) => {
  const id = String(req.body.id || '')
  const name = String(req.body.name || '')
  const size = Number(req.body.size || 0)
  if (!id || !name || !size) return res.status(400).json({ error: 'param' })
  const dir = path.join(root, '.chunks', id)
  fs.mkdirSync(dir, { recursive: true })
  fs.writeFileSync(path.join(dir, 'meta.json'), JSON.stringify({ id, name, size }))
  res.json({ ok: true })
})

router.post('/upload/chunk', express.raw({ type: '*/*', limit: '100mb' }), (req, res) => {
  const id = String(req.query.id || '')
  const index = String(req.query.index || '')
  if (!id || !index) return res.status(400).json({ error: 'param' })
  const dir = path.join(root, '.chunks', id)
  fs.mkdirSync(dir, { recursive: true })
  fs.writeFileSync(path.join(dir, index), req.body)
  res.json({ ok: true })
})

router.post('/upload/finish', (req, res) => {
  const id = String(req.body.id || '')
  if (!id) return res.status(400).json({ error: 'id' })
  const dir = path.join(root, '.chunks', id)
  const meta = JSON.parse(fs.readFileSync(path.join(dir, 'meta.json')).toString())
  const target = path.join(root, meta.name)
  const files = fs.readdirSync(dir).filter(n => n !== 'meta.json').sort((a, b) => Number(a) - Number(b))
  const w = fs.createWriteStream(target)
  for (const f of files) {
    const buf = fs.readFileSync(path.join(dir, f))
    w.write(buf)
  }
  w.end()
  res.json({ ok: true })
})

router.get('/upload/state', (req, res) => {
  const id = String(req.query.id || '')
  if (!id) return res.status(400).json({ error: 'id' })
  const dir = path.join(root, '.chunks', id)
  if (!fs.existsSync(dir)) return res.json({ chunks: [] })
  const files = fs.readdirSync(dir).filter(n => n !== 'meta.json')
  res.json({ chunks: files })
})

export default router