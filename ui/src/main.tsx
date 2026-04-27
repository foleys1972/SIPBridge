import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './app/App'
import { AuthProvider } from './app/auth/AuthContext'
import LoginGate from './app/auth/LoginGate'
import './styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AuthProvider>
      <BrowserRouter>
        <LoginGate>
          <App />
        </LoginGate>
      </BrowserRouter>
    </AuthProvider>
  </React.StrictMode>,
)
