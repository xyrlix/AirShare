import React, { useEffect, useState } from 'react'

type Dev = { name: string, addr: string, port: number, meta: string[] }

export default function LanDevices() {
  const [list, setList] = useState<Dev[]>([])
  useEffect(() => {
    fetch('http://localhost:8444/api/devices').then(r => r.json()).then(setList).catch(() => {})
  }, [])
  return (
    <div>
      <div>局域网设备</div>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr><th>名称</th><th>地址</th><th>端口</th><th>指纹</th><th>操作</th></tr>
        </thead>
        <tbody>
          {list.map((d, i) => {
            const fp = (d.meta || []).find(x => x.startsWith('fp='))?.slice(3) || ''
            return (
              <tr key={i}>
                <td>{d.name}</td>
                <td>{d.addr}</td>
                <td>{d.port}</td>
                <td>{fp}</td>
                <td><a href={`https://${d.addr}:8443/?fp=${fp}`} target="_blank" rel="noreferrer">打开</a></td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
