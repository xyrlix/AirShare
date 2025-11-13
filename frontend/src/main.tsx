import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './i18n'
if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js')

const el = document.getElementById('root')!
createRoot(el).render(<App />)