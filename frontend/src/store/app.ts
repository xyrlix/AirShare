import { create } from 'zustand'

type State = {
  room: string
  setRoom: (v: string) => void
  peers: { id: string, name?: string }[]
  setPeers: (p: { id: string, name?: string }[]) => void
}

export const useApp = create<State>(set => ({
  room: '',
  setRoom: v => set({ room: v }),
  peers: [],
  setPeers: p => set({ peers: p })
}))
