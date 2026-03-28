import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { ThemeProvider } from './contexts/ThemeContext'
import { CompactModeProvider } from './contexts/CompactModeContext'
import { ToastProvider } from './contexts/ToastContext'
import ToastContainer from './components/ToastContainer'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider>
      <CompactModeProvider>
        <ToastProvider>
          <App />
          <ToastContainer />
        </ToastProvider>
      </CompactModeProvider>
    </ThemeProvider>
  </StrictMode>,
)
