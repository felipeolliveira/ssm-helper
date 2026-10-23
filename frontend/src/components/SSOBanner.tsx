import {SSOCode} from '../api'
import {BrowserOpenURL, ClipboardSetText} from '../../wailsjs/runtime/runtime'
import {Icon} from './Icon'

// Shown while an SSO login waits for confirmation in the browser.
export default function SSOBanner({codes, onCancel}: {codes: SSOCode[], onCancel: (profile: string) => void}) {
    if (codes.length === 0) return null
    return (
        <div className="sso-banners">
            {codes.map(({profile, code}) => (
                <div key={profile} className="alert info sso-banner">
                    <Icon name="key" size={18}/>
                    <div className="sso-text">
                        <strong>Login SSO do perfil {profile}</strong>
                        <span>Confirme no navegador o código</span>
                    </div>
                    <code className="sso-code">{code.userCode}</code>
                    <button className="btn ghost" onClick={() => ClipboardSetText(code.userCode)} title="Copiar código">
                        <Icon name="copy"/>
                    </button>
                    <button className="btn" onClick={() => BrowserOpenURL(code.verificationUriComplete)}>
                        <Icon name="globe"/> Abrir página de login
                    </button>
                    <button className="btn ghost" onClick={() => onCancel(profile)}>Cancelar</button>
                </div>
            ))}
        </div>
    )
}
