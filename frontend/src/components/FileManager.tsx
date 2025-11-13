import React, { useEffect, useRef, useState } from 'react'

type Item = { name: string, isDir: boolean }

export function FileManager({ base }: { base: string }) {
  const [list, setList] = useState<Item[]>([])
  const inputRef = useRef<HTMLInputElement>(null)
  useEffect(() => { load() }, [])
  function load() {
    fetch(`${base}`).then(r => r.json()).then(setList)
  }
  async function uploadFile(f: File) {
    const id = crypto.randomUUID()
    await fetch(`${base}/upload/init`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id, name: f.name, size: f.size }) })
    const size = 1024 * 1024
    let i = 0
    while (i < f.size) {
      const chunk = f.slice(i, i + size)
      const buf = await chunk.arrayBuffer()
      await fetch(`${base}/upload/chunk?id=${id}&index=${i / size}`, { method: 'POST', body: buf })
      i += size
    }
    await fetch(`${base}/upload/finish`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id }) })
    load()
  }
  function onDrop(e: React.DragEvent) {
    e.preventDefault()
    const f = e.dataTransfer.files?.[0]
    if (f) uploadFile(f)
  }
  return (
    <div onDragOver={e => e.preventDefault()} onDrop={onDrop} style={{ border: '1px dashed #999', padding: 8 }}>
      <div>
        <button onClick={() => inputRef.current?.click()}>选择上传</button>
        <input ref={inputRef} type="file" style={{ display: 'none' }} onChange={e => { const f = e.target.files?.[0]; if (f) uploadFile(f) }} />
      </div>
      <ul>
        {list.map(i => <li key={i.name}><a href={`${base}/download?name=${encodeURIComponent(i.name)}`}>{i.name}</a></li>)}
      </ul>
    </div>
  )
}