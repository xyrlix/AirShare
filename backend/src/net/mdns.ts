import mdns from 'multicast-dns'

export function createMdns(port: number) {
  const m = mdns()
  const name = 'AirShare'
  const type = '_airshare._tcp.local'
  m.on('query', q => {
    for (const question of q.questions) {
      if (question.name === type) {
        m.respond({ answers: [{ name: type, type: 'SRV', data: { port, target: name } }] })
      }
    }
  })
  setInterval(() => {
    m.respond({ answers: [{ name: type, type: 'SRV', data: { port, target: name } }] })
  }, 5000)
}