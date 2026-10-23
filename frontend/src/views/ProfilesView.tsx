import {useState} from 'react'
import {api, Env, localMessage, Message, ProfileStatus, toMessage} from '../api'
import {Icon} from '../components/Icon'
import {Secret, useSecretText} from '../privacy'
import {useI18n} from '../i18n'

export default function ProfilesView({env, statuses, onRefresh}: {
    env: Env
    statuses: ProfileStatus[]
    onRefresh: () => void
}) {
    const {t, tm, formatDateTime} = useI18n()
    const [pending, setPending] = useState<Record<string, boolean>>({})
    const secretText = useSecretText()
    const [messages, setMessages] = useState<Record<string, {message: Message | null, text: string, error: boolean}>>({})

    const login = async (name: string) => {
        setPending(p => ({...p, [name]: true}))
        setMessages(m => ({...m, [name]: {message: null, text: t('profiles.loginPending'), error: false}}))
        try {
            await api.login(name)
            setMessages(m => ({...m, [name]: {message: localMessage('SSO_LOGIN_SUCCEEDED', {profile: name}), text: '', error: false}}))
        } catch (err) {
            setMessages(m => ({...m, [name]: {message: toMessage(err), text: '', error: true}}))
        } finally {
            setPending(p => ({...p, [name]: false}))
            onRefresh()
        }
    }

    return (
        <div className="page">
            <header className="page-header">
                <div>
                    <h1>{t('profiles.title')}</h1>
                    <p className="muted">{t('profiles.subtitle')}</p>
                </div>
                <div className="spacer"/>
                <button className="btn" onClick={onRefresh}><Icon name="refresh"/> {t('common.refresh')}</button>
            </header>
            <div className="table-card card">
                <table>
                    <thead>
                    <tr>
                        <th>{t('profiles.profile')}</th>
                        <th>{t('profiles.account')}</th>
                        <th>{t('profiles.role')}</th>
                        <th>{t('profiles.region')}</th>
                        <th>{t('profiles.session')}</th>
                        <th/>
                    </tr>
                    </thead>
                    <tbody>
                    {env.profiles.map(profile => {
                        const status = statuses.find(s => s.name === profile.name)
                        const message = messages[profile.name]
                        return (
                            <tr key={profile.name}>
                                <td><strong>{profile.name}</strong>
                                    {message && (
                                        <div className={`small ${message.error ? `text-error ${secretText}` : 'muted'}`}>
                                            {message.message ? tm(message.message) : message.text}
                                        </div>
                                    )}
                                </td>
                                <td><code><Secret value={profile.accountId} keepLast={4}/></code></td>
                                <td>{profile.roleName}</td>
                                <td>{profile.ssoRegion}</td>
                                <td>
                                    {status?.loggedIn
                                        ? <span className="status-pill connected"><span className="dot"/>{t('profiles.validUntil', {date: formatDateTime(status.expiresAt)})}</span>
                                        : <span className="status-pill idle"><span className="dot"/>{t('profiles.expired')}</span>}
                                </td>
                                <td className="right">
                                    {pending[profile.name] ? (
                                        <button className="btn ghost" onClick={() => api.cancelLogin(profile.name)}>{t('common.cancel')}</button>
                                    ) : (
                                        <button className="btn" onClick={() => login(profile.name)}>
                                            <Icon name="key"/> {t('profiles.login')}
                                        </button>
                                    )}
                                </td>
                            </tr>
                        )
                    })}
                    </tbody>
                </table>
            </div>
        </div>
    )
}
