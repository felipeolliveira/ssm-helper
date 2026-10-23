import {useState} from 'react'
import {api, Env, errorMessage, ProfileStatus} from '../api'
import {Icon} from '../components/Icon'

function formatExpiry(iso: string) {
    if (!iso) return ''
    const date = new Date(iso)
    return date.toLocaleString('pt-BR', {day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit'})
}

export default function ProfilesView({env, statuses, onRefresh}: {
    env: Env
    statuses: ProfileStatus[]
    onRefresh: () => void
}) {
    const [pending, setPending] = useState<Record<string, boolean>>({})
    const [messages, setMessages] = useState<Record<string, {text: string, error: boolean}>>({})

    const login = async (name: string) => {
        setPending(p => ({...p, [name]: true}))
        setMessages(m => ({...m, [name]: {text: 'Conclua o login no navegador...', error: false}}))
        try {
            await api.login(name)
            setMessages(m => ({...m, [name]: {text: `Perfil '${name}' autenticado via SSO com sucesso!`, error: false}}))
        } catch (err) {
            setMessages(m => ({...m, [name]: {text: errorMessage(err), error: true}}))
        } finally {
            setPending(p => ({...p, [name]: false}))
            onRefresh()
        }
    }

    return (
        <div className="page">
            <header className="page-header">
                <div>
                    <h1>Perfis AWS</h1>
                    <p className="muted">Perfis com a mesma URL de SSO compartilham o mesmo login.</p>
                </div>
                <div className="spacer"/>
                <button className="btn" onClick={onRefresh}><Icon name="refresh"/> Atualizar</button>
            </header>
            <div className="table-card card">
                <table>
                    <thead>
                    <tr>
                        <th>Perfil</th>
                        <th>Conta</th>
                        <th>Role</th>
                        <th>Região</th>
                        <th>Sessão SSO</th>
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
                                    {message && <div className={`small ${message.error ? 'text-error' : 'muted'}`}>{message.text}</div>}
                                </td>
                                <td><code>{profile.accountId}</code></td>
                                <td>{profile.roleName}</td>
                                <td>{profile.ssoRegion}</td>
                                <td>
                                    {status?.loggedIn
                                        ? <span className="status-pill connected"><span className="dot"/>Válida até {formatExpiry(status.expiresAt)}</span>
                                        : <span className="status-pill idle"><span className="dot"/>Expirada</span>}
                                </td>
                                <td className="right">
                                    {pending[profile.name] ? (
                                        <button className="btn ghost" onClick={() => api.cancelLogin(profile.name)}>Cancelar</button>
                                    ) : (
                                        <button className="btn" onClick={() => login(profile.name)}>
                                            <Icon name="key"/> Login SSO
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
