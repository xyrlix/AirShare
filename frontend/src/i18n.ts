import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

const resources = {
  zh: { translation: { devices: '设备', files: '文件', transfer: '传输', selectUpload: '选择上传', refresh: '刷新设备', room: '房间', self: '自己', selectPeer: '选择设备', connect: '连接', sendDone: '发送完成' } },
  en: { translation: { devices: 'Devices', files: 'Files', transfer: 'Transfer', selectUpload: 'Select Upload', refresh: 'Refresh', room: 'Room', self: 'Self', selectPeer: 'Select Device', connect: 'Connect', sendDone: 'Send Done' } }
}

i18n.use(initReactI18next).init({ resources, lng: 'zh', fallbackLng: 'en', interpolation: { escapeValue: false } })

export default i18n