import React, { useEffect, useState } from 'react'

export default function FileManager({ baseUrl, onChanged }: { baseUrl: string, onChanged: () => void }) {
  const [list, setList] = useState<{ name: string, size: number }[]>([])
  const [rename, setRename] = useState<Record<string, string>>({})
  async function load() {
    const j = await fetch(`${baseUrl}/api/files/list`).then(r => r.json())
    setList(j.files || [])
  }
  useEffect(() => { load() }, [baseUrl])
  async function upload(fs: FileList) {
    for (let i = 0; i < fs.length; i++) {
      const f = fs[i]
      const id = crypto.randomUUID()
      const chunkSize = 256 * 1024
      const total = Math.ceil(f.size / chunkSize)
      await fetch(`${baseUrl}/api/files/upload/init`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id, name: f.name, size: f.size }) })
      for (let j = 0; j < total; j++) {
        const start = j * chunkSize
        const end = Math.min(start + chunkSize, f.size)
        const buf = await f.slice(start, end).arrayBuffer()
        const form = new FormData()
        form.append('bin', new Blob([buf]))
        await fetch(`${baseUrl}/api/files/upload/chunk?id=${id}&index=${j}`, { method: 'POST', body: form })
      }
      await fetch(`${baseUrl}/api/files/upload/finish`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id, name: f.name, total }) })
    }
    await load()
    onChanged()
  }
  async function del(name: string) {
    await fetch(`${baseUrl}/api/files/delete?name=${encodeURIComponent(name)}`, { method: 'DELETE' })
    await load()
    onChanged()
  }
  async function ren(from: string) {
    const to = rename[from] || ''
    if (!to) return
    await fetch(`${baseUrl}/api/files/rename`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ from, to }) })
    setRename(s => ({ ...s, [from]: '' }))
    await load()
    onChanged()
  }
  return (
    <div>
      <div>
        <input type="file" multiple onChange={e => { const fs = e.target.files; if (fs && fs.length) upload(fs) }} />
      </div>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr><th>名称</th><th>大小</th><th>操作</th></tr>
        </thead>
        <tbody>
          {list.map(f => (
            <tr key={f.name}>
              <td>{f.name}</td>
              <td>{Math.round(f.size / 1024)}KB</td>
              <td>
                <a href={`${baseUrl}/api/files/download?name=${encodeURIComponent(f.name)}`} target="_blank" rel="noreferrer">下载</a>
                <button onClick={() => del(f.name)} style={{ marginLeft: 8 }}>删除</button>
                <input value={rename[f.name] || ''} onChange={e => setRename(s => ({ ...s, [f.name]: e.target.value }))} placeholder="重命名为" style={{ marginLeft: 8 }} />
                <button onClick={() => ren(f.name)} style={{ marginLeft: 4 }}>重命名</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}