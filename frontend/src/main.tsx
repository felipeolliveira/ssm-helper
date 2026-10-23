import React from 'react'
import {createRoot} from 'react-dom/client'
import './fonts'
import './style.css'
import App from './App'
import {PrivacyProvider} from './privacy'

createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <PrivacyProvider>
            <App/>
        </PrivacyProvider>
    </React.StrictMode>
)
