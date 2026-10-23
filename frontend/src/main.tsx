import React from 'react'
import {createRoot} from 'react-dom/client'
import './fonts'
import './style.css'
import App from './App'
import {PrivacyProvider} from './privacy'
import {I18nProvider} from './i18n'

createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <I18nProvider>
            <PrivacyProvider>
                <App/>
            </PrivacyProvider>
        </I18nProvider>
    </React.StrictMode>
)
