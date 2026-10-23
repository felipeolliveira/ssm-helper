import {useState} from 'react'
import {api, Env, errorMessage, ProfileStatus, Target, TunnelState, TunnelStatus, tunnelKey} from '../api'
import {Icon} from '../components/Icon'
import {Secret, useSecretText} from '../privacy'

const statusLabels: Record<TunnelStatus, string> = {
    idle: 'Desconectado',
    connecting: 'Conectando',
    login: 'Aguardando login',
    connected: 'Conectado',
    error: 'Erro',
}

function linkToURL(link: string) {
    return /^https?:\/\//.test(link) ? link : `https://${link}`
}

export default function TargetsView({envName, env, tunnels, profiles, autoChrome, onAutoChromeChange}: {
    envName: string
    env: Env
    // All tunnels, keyed by tunnelKey(env, target)
    tunnels: Record<string, TunnelState>
    profiles: ProfileStatus[]
    autoChrome: boolean
    onAutoChromeChange: (value: boolean) => void
}) {
    return (
        <div className="page">
            <header className="page-header">
                <div>
                    <h1>Targets</h1>
                    <p className="muted">Conecte a um ambiente para abrir o túnel SSM na porta local.</p>
                </div>
                <div className="spacer"/>
                <label className="switch" title="Abre o Chrome assim que o túnel ficar pronto, para targets com domínios ou URLs">
                    <input type="checkbox" checked={autoChrome} onChange={e => onAutoChromeChange(e.target.checked)}/>
                    <span className="switch-track"/>
                    <span>Abrir o Chrome ao conectar</span>
                </label>
            </header>
            <div className="card-grid">
                {env.targets.map(target => (
                    <TargetCard
                        key={target.name}
                        target={target}
                        envName={envName}
                        state={tunnels[tunnelKey(envName, target.name)]}
                        loggedIn={profiles.find(p => p.name === target.profile)?.loggedIn}
                        region={target.ssmRegion || env.profiles.find(p => p.name === target.profile)?.ssoRegion || ''}
                        autoChrome={autoChrome}
                    />
                ))}
            </div>
        </div>
    )
}

function TargetCard({envName, target, state, loggedIn, region, autoChrome}: {
    envName: string
    target: Target
    state?: TunnelState
    loggedIn?: boolean
    region: string
    autoChrome: boolean
}) {
    const [error, setError] = useState('')
    const status: TunnelStatus = state?.status ?? 'idle'
    const busy = status === 'connecting' || status === 'login'
    const connected = status === 'connected'
    const usesBrowser = target.dnsHosts.length > 0 || target.openUrls.length > 0
    const secretText = useSecretText()
    const hidden = secretText !== ''

    const run = async (action: () => Promise<void>) => {
        setError('')
        try {
            await action()
        } catch (err) {
            setError(errorMessage(err))
        }
    }

    return (
        <article className={`card target-card status-${status}`}>
            <header className="card-header">
                <div>
                    <h2>{target.name}</h2>
                    <div className="muted small">
                        perfil <strong>{target.profile}</strong>
                        {loggedIn === false && <span className="tag warn">login necessário</span>}
                    </div>
                </div>
                <span className={`status-pill ${status}`}>
                    <span className="dot"/>{statusLabels[status]}
                </span>
            </header>

            <dl className="facts">
                <div><dt>Porta</dt><dd><code>{target.remotePort} → localhost:{target.localPort}</code></dd></div>
                <div><dt>Região</dt><dd>{region}</dd></div>
                <div className="wide"><dt>Host remoto</dt><dd className="truncate" title={hidden ? undefined : target.remoteHost}><Secret value={target.remoteHost}/></dd></div>
                {target.dnsHosts.length > 0 && (
                    <div className="wide"><dt>Domínios</dt><dd className="chips">
                        {target.dnsHosts.map(h => <span key={h} className="chip"><Secret value={h}/></span>)}
                    </dd></div>
                )}
                {target.openUrls.length > 0 && (
                    <div className="wide"><dt>URLs</dt><dd className="chips">
                        {target.openUrls.map(u => <span key={u} className="chip"><Secret value={linkToURL(u)}/></span>)}
                    </dd></div>
                )}
            </dl>

            {(state?.message || error) && (
                <p className={`card-message ${status === 'error' || error ? `error ${secretText}` : ''}`}>{error || state?.message}</p>
            )}

            <footer className="card-actions">
                {connected || busy ? (
                    <button className="btn danger" onClick={() => run(() => api.disconnect(envName, target.name))}>
                        <Icon name="stop"/> {busy ? 'Cancelar' : 'Desconectar'}
                    </button>
                ) : (
                    <button className="btn primary" onClick={() => run(() => api.connect(target.name, autoChrome))}>
                        <Icon name="play"/> Conectar
                    </button>
                )}
                {usesBrowser && (
                    <button className="btn" disabled={!connected} onClick={() => run(() => api.openChrome(target.name))}
                            title={connected ? 'Abre o Chrome apontando para o túnel' : 'Conecte primeiro'}>
                        <Icon name="globe"/> Abrir Chrome
                    </button>
                )}
            </footer>
        </article>
    )
}
