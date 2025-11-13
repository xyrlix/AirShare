type Callbacks = {
  onCtrl: (msg: any) => void
  onData: (buf: ArrayBuffer) => void
  onText: (text: string) => void
  onSignal: (obj: any) => void
}

export function createPeer(ws: WebSocket, to: string, cbs: Callbacks) {
  const pc = new RTCPeerConnection()
  pc.onicecandidate = ev => { if (ev.candidate) ws.send(JSON.stringify({ t: 'signal', to, data: { type: 'candidate', candidate: ev.candidate } })) }
  const ctrl = pc.createDataChannel('ctrl')
  const data = pc.createDataChannel('data')
  const text = pc.createDataChannel('text')
  data.binaryType = 'arraybuffer'
  ctrl.onmessage = e => cbs.onCtrl(JSON.parse(String(e.data)))
  data.onmessage = e => cbs.onData(e.data as ArrayBuffer)
  text.onmessage = e => cbs.onText(String(e.data))
  pc.ondatachannel = ev => {
    if (ev.channel.label === 'ctrl') ev.channel.onmessage = e => cbs.onCtrl(JSON.parse(String(e.data)))
    if (ev.channel.label === 'data') { ev.channel.binaryType = 'arraybuffer'; ev.channel.onmessage = e => cbs.onData(e.data as ArrayBuffer) }
    if (ev.channel.label === 'text') ev.channel.onmessage = e => cbs.onText(String(e.data))
  }
  pc.createOffer().then(o => pc.setLocalDescription(o).then(() => ws.send(JSON.stringify({ t: 'signal', to, data: o }))))
  return { pc, ctrl, data, text }
}
