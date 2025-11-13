export type RecordItem = { id: string, name: string, size: number, peer?: string, time: number, dir: 'out' | 'in' }
const k = 'airshare-history'
export function addRecord(r: RecordItem) {
  const a = getRecords()
  a.unshift(r)
  localStorage.setItem(k, JSON.stringify(a.slice(0, 200)))
}
export function getRecords(): RecordItem[] {
  const s = localStorage.getItem(k)
  return s ? JSON.parse(s) : []
}