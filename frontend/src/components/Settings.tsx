import React from 'react'
import { useSettings } from '../store/settings'
import i18n from '../i18n'

export default function Settings() {
  const { bandwidthKBps, strategy, encrypt, language, setBandwidth, setStrategy, setEncrypt, setLanguage } = useSettings()
  React.useEffect(() => { i18n.changeLanguage(language) }, [language])
  return (
    <div>
      <div>设置</div>
      <div>
        <label>带宽上限(KB/s)</label>
        <input type="number" value={bandwidthKBps} onChange={e => setBandwidth(Number(e.target.value || 0))} />
      </div>
      <div>
        <label>窗口策略</label>
        <select value={strategy} onChange={e => setStrategy(e.target.value as any)}>
          <option value="vegas">Vegas</option>
          <option value="cubic">Cubic</option>
        </select>
      </div>
      <div>
        <label>内容加密</label>
        <input type="checkbox" checked={encrypt} onChange={e => setEncrypt(e.target.checked)} />
      </div>
      <div>
        <label>语言</label>
        <select value={language} onChange={e => setLanguage(e.target.value as any)}>
          <option value="zh-CN">中文</option>
          <option value="en">English</option>
        </select>
      </div>
    </div>
  )
}
