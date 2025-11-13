import dgram from 'dgram'

export function createUdpDiscovery(port: number) {
  const sock = dgram.createSocket('udp4')
  const group = '239.255.255.250'
  sock.bind(53317, () => {
    sock.addMembership(group)
  })
  sock.on('message', (msg, rinfo) => {
    const s = msg.toString()
    if (s === 'AIRSHARE_DISCOVERY') {
      const reply = Buffer.from(JSON.stringify({ service: 'airshare', port }))
      sock.send(reply, 0, reply.length, rinfo.port, rinfo.address)
    }
  })
  setInterval(() => {
    const buf = Buffer.from('AIRSHARE_PRESENCE')
    sock.send(buf, 0, buf.length, 53317, group)
  }, 3000)
}