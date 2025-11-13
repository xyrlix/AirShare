import React from 'react'
import i18n from '../i18n'

export default function Language() {
  const [lng, setLng] = React.useState(i18n.language)
  function change(v: string) { i18n.changeLanguage(v); setLng(v) }
  return (
    <div>
      <select value={lng} onChange={e => change(e.target.value)}>
        <option value="zh-CN">中文</option>
        <option value="en">English</option>
      </select>
    </div>
  )
}
