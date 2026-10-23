import {SSOCode} from '../api'
import {BrowserOpenURL, ClipboardSetText} from '../../wailsjs/runtime/runtime'
import {Icon} from './Icon'
import {useI18n} from '../i18n'

// Shown while an SSO login waits for confirmation in the browser.
export default function SSOBanner({codes, onCancel}: {codes: SSOCode[], onCancel: (profile: string) => void}) {
    const {t} = useI18n()
    if (codes.length === 0) return null
    return (
        <div className="sso-banners">
            {codes.map(({profile, code}) => (
                <div key={profile} className="alert info sso-banner">
                    <Icon name="key" size={18}/>
                    <div className="sso-text">
                        <strong>{t('sso.bannerTitle', {profile})}</strong>
                        <span>{t('sso.bannerText')}</span>
                    </div>
                    <code className="sso-code">{code.userCode}</code>
                    <button className="btn ghost" onClick={() => ClipboardSetText(code.userCode)} title={t('sso.copyCode')}>
                        <Icon name="copy"/>
                    </button>
                    <button className="btn" onClick={() => BrowserOpenURL(code.verificationUriComplete)}>
                        <Icon name="globe"/> {t('sso.openLoginPage')}
                    </button>
                    <button className="btn ghost" onClick={() => onCancel(profile)}>{t('common.cancel')}</button>
                </div>
            ))}
        </div>
    )
}
