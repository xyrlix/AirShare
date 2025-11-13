import React, { useEffect, useState } from 'react'
import { getRecords } from '@/services/history'

export function History() {
  const [list, setList] = useState(getRecords())
  useEffect(() => { const id = setInterval(() => setList(getRecords()), 1000); return () => clearInterval(id) }, [])
  return (
    <div>
      <h3>历史</h3>
      <ul>
        {list.map(r => <li key={r.id}>{r.dir} {r.name} {Math.round(r.size/1024)}KB {r.peer}</li>)}
      </ul>
    </div>
  )
}