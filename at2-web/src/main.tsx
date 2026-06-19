import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './i18n'
import { App } from './app.tsx'
import { registerServiceWorker } from './push'

// Register the push service worker (no-op where unsupported).
registerServiceWorker()

createRoot(document.getElementById('app')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
