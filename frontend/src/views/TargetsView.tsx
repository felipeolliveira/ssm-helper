import {useState} from 'react'
import {api, Env, Message, ProfileStatus, Target, toMessage, TunnelState, TunnelStatus, tunnelKey} from '../api'
import {Icon} from '../components/Icon'
import {Secret, useSecretText} from '../privacy'
import {useI18n} from '../i18n'

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
    const {t} = useI18n()
    return (
        <div className="page">
            <header className="page-header">
                <div>
                    <h1>{t('targets.title')}</h1>
                    <p className="muted">{t('targets.subtitle')}</p>
                </div>
                <div className="spacer"/>
                <label className="switch" title={t('targets.openChromeOnConnectHint')}>
                    <input type="checkbox" checked={autoChrome} onChange={e => onAutoChromeChange(e.target.checked)}/>
                    <span className="switch-track"/>
                    <span>{t('targets.openChromeOnConnect')}</span>
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
    const {t, tm} = useI18n()
    const [error, setError] = useState<Message | null>(null)
    const status: TunnelStatus = state?.status ?? 'idle'
    const busy = status === 'connecting' || status === 'login'
    const connected = status === 'connected'
    const usesBrowser = target.dnsHosts.length > 0 || target.openUrls.length > 0
    const secretText = useSecretText()
    const hidden = secretText !== ''
    const message = error ?? (state?.message?.code ? state.message : null)

    const run = async (action: () => Promise<void>) => {
        setError(null)
        try {
            await action()
        } catch (err) {
            setError(toMessage(err))
        }
    }

    return (
        <article className={`card target-card status-${status}`}>
            <header className="card-header">
                <div>
                    <h2>{target.name}</h2>
                    <div className="muted small">
                        {t('targets.profile')} <strong>{target.profile}</strong>
                        {loggedIn === false && <span className="tag warn">{t('targets.loginNeeded')}</span>}
                    </div>
                </div>
                <span className={`status-pill ${status}`}>
                    <span className="dot"/>{t(`status.${status}`)}
                </span>
            </header>

            <dl className="facts">
                <div><dt>{t('targets.port')}</dt><dd><code>{target.remotePort} → localhost:{target.localPort}</code></dd></div>
                <div><dt>{t('targets.region')}</dt><dd>{region}</dd></div>
                <div className="wide"><dt>{t('targets.remoteHost')}</dt><dd className="truncate" title={hidden ? undefined : target.remoteHost}><Secret value={target.remoteHost}/></dd></div>
                {target.dnsHosts.length > 0 && (
                    <div className="wide"><dt>{t('targets.domains')}</dt><dd className="chips">
                        {target.dnsHosts.map(h => <span key={h} className="chip"><Secret value={h}/></span>)}
                    </dd></div>
                )}
                {target.openUrls.length > 0 && (
                    <div className="wide"><dt>{t('targets.urls')}</dt><dd className="chips">
                        {target.openUrls.map(u => <span key={u} className="chip"><Secret value={linkToURL(u)}/></span>)}
                    </dd></div>
                )}
            </dl>

            {message && (
                <p className={`card-message ${status === 'error' || error ? `error ${secretText}` : ''}`}>{tm(message)}</p>
            )}

            <footer className="card-actions">
                {connected || busy ? (
                    <button className="btn danger" onClick={() => run(() => api.disconnect(envName, target.name))}>
                        <Icon name="stop"/> {busy ? t('common.cancel') : t('targets.disconnect')}
                    </button>
                ) : (
                    <button className="btn primary" onClick={() => run(() => api.connect(target.name, autoChrome))}>
                        <Icon name="play"/> {t('targets.connect')}
                    </button>
                )}
                {usesBrowser && (
                    <button className="btn" disabled={!connected} onClick={() => run(() => api.openChrome(target.name))}
                            title={connected ? t('targets.openChromeHint') : t('targets.connectFirst')}>
                        <Icon name="globe"/> {t('targets.openChrome')}
                    </button>
                )}
            </footer>
        </article>
    )
}
