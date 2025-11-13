import crypto from 'crypto'

const secret = process.env.ROOM_SECRET || 'airshare-secret'

export function createRoomSigned(room: string) {
  const token = crypto.randomBytes(16).toString('hex')
  const exp = Date.now() + 10 * 60 * 1000
  const sig = crypto.createHmac('sha256', secret).update(`${room}|${token}|${exp}`).digest('hex')
  return { token, exp, sig }
}

export function verifySigned(room: string, token: string, exp: number, sig: string) {
  if (!room || !token || !sig || !exp) return false
  if (Date.now() > exp) return false
  const expect = crypto.createHmac('sha256', secret).update(`${room}|${token}|${exp}`).digest('hex')
  return expect === sig
}
