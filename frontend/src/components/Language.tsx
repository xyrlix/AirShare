import React from 'react'
import i18n from '@/i18n'

export function Language() {
  function set(l: string) { i18n.changeLanguage(l) }
  return (
    <div>
      <button onClick={() => set('zh')}>中文</button>
      <button onClick={() => set('en')}>EN</button>
    </div>
  )
}