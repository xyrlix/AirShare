import React, { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

type Dev = { name: string, addr: string, port: number, meta: string[] }

export default function LanDevices() {
  const [list, setList] = useState<Dev[]>([])
  const { t } = useTranslation()
  useEffect(() => {
    fetch('http://localhost:8444/api/devices').then(r => r.json()).then(setList).catch(() => {})
  }, [])
  return (
    <div>
      <div>{t('lan_devices')}</div>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr><th>{t('name')}</th><th>{t('address')}</th><th>{t('port')}</th><th>{t('fingerprint')}</th><th>{t('action')}</th></tr>
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
                <td><a href={`https://${d.addr}:8443/?fp=${fp}`} target="_blank" rel="noreferrer">{t('open')}</a></td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
