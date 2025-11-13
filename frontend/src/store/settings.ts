import { create } from 'zustand'

type Settings = {
  bandwidthKBps: number
  strategy: 'vegas' | 'cubic'
  encrypt: boolean
  language: 'zh-CN' | 'en'
  setBandwidth: (v: number) => void
  setStrategy: (v: 'vegas' | 'cubic') => void
  setEncrypt: (v: boolean) => void
  setLanguage: (v: 'zh-CN' | 'en') => void
}

const KEY = 'airshare_settings'
function load() {
  try { const s = localStorage.getItem(KEY); return s ? JSON.parse(s) : {} } catch { return {} }
}
function save(obj: any) { try { localStorage.setItem(KEY, JSON.stringify(obj)) } catch {} }

export const useSettings = create<Settings>(set => {
  const init = Object.assign({ bandwidthKBps: 0, strategy: 'vegas', encrypt: true, language: 'zh-CN' }, load())
  return {
    bandwidthKBps: init.bandwidthKBps,
    strategy: init.strategy,
    encrypt: init.encrypt,
    language: init.language,
    setBandwidth: v => set(s => { const n = { ...s, bandwidthKBps: v } as any; save(n); return n }),
    setStrategy: v => set(s => { const n = { ...s, strategy: v } as any; save(n); return n }),
    setEncrypt: v => set(s => { const n = { ...s, encrypt: v } as any; save(n); return n }),
    setLanguage: v => set(s => { const n = { ...s, language: v } as any; save(n); return n })
  }
})
