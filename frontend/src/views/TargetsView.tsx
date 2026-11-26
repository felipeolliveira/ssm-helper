import {useState} from 'react'
import {api, Env, Message, ProfileStatus, Target, toMessage, TunnelState, TunnelStatus, tunnelKey} from '../api'
import ActionsMenu from '../components/ActionsMenu'
import ConfirmDialog from '../components/ConfirmDialog'
import {Icon} from '../components/Icon'
import {ClipboardSetText} from '../../wailsjs/runtime/runtime'
import {Secret, useSecretText} from '../privacy'
import {useI18n} from '../i18n'

// Copies the local address of a tunnel, for curl, Postman or a database client.
function CopyButton({text}: {text: string}) {
    const {t} = useI18n()
    const [copied, setCopied] = useState(false)
    const copy = async () => {
        if (await ClipboardSetText(text)) {
            setCopied(true)
            setTimeout(() => setCopied(false), 1500)
        }
    }
    return (
        <button className={`btn ghost icon-only copy-inline ${copied ? 'copied' : ''}`} onClick={copy}
                title={copied ? t('targets.addressCopied') : t('targets.copyAddress', {address: text})}
                aria-label={t('targets.copyAddress', {address: text})}>
            <Icon name={copied ? 'check' : 'copy'} size={13}/>
        </button>
    )
}

function linkToURL(link: string) {
    return /^https?:\/\//.test(link) ? link : `https://${link}`
}

export default function TargetsView({envName, env, tunnels, profiles, autoChrome, chromeOpen, onAutoChromeChange}: {
    envName: string
    env: Env
    // All tunnels, keyed by tunnelKey(env, target)
    tunnels: Record<string, TunnelState>
    profiles: ProfileStatus[]
    autoChrome: boolean
    // Chrome profiles with a window the app opened
    chromeOpen: string[]
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
                        chromeIsOpen={chromeOpen.includes(chromeProfile(target))}
                        sharedWith={env.targets
                            .filter(o => o.name !== target.name && chromeProfile(o) === chromeProfile(target))
                            .map(o => o.name)}
                    />
                ))}
            </div>
        </div>
    )
}

// The Chrome profile folder of a target; empty chromeProfile falls back to the name, as in the backend.
function chromeProfile(target: Target) {
    return target.chromeProfile || target.name
}

function TargetCard({envName, target, state, loggedIn, region, autoChrome, chromeIsOpen, sharedWith}: {
    envName: string
    target: Target
    state?: TunnelState
    loggedIn?: boolean
    region: string
    autoChrome: boolean
    chromeIsOpen: boolean
    // Other targets of the environment with the same Chrome profile
    sharedWith: string[]
}) {
    const {t, tm} = useI18n()
    const [error, setError] = useState<Message | null>(null)
    const [confirmClear, setConfirmClear] = useState(false)
    const [clearing, setClearing] = useState(false)
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

    const clearBrowserData = async () => {
        setConfirmClear(false)
        setClearing(true)
        await run(() => api.clearChromeProfile(target.name))
        setClearing(false)
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
                <div className="card-header-end">
                    <span className={`status-pill ${status}`}>
                        <span className="dot"/>{t(`status.${status}`)}
                    </span>
                    {usesBrowser && (
                        <ActionsMenu label={t('targets.actions')} disabled={clearing} actions={[
                            {label: t('targets.clearBrowserData'), icon: 'eraser', danger: true, onSelect: () => setConfirmClear(true)},
                        ]}/>
                    )}
                </div>
            </header>

            <dl className="facts">
                <div><dt>{t('targets.port')}</dt><dd>
                    <code>{target.remotePort} → localhost:{target.localPort}</code>
                    <CopyButton text={`localhost:${target.localPort}`}/>
                </dd></div>
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

            {clearing && <p className="card-message">{t('targets.clearingBrowserData')}</p>}
            {message && !clearing && (
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
                {usesBrowser && (chromeIsOpen ? (
                    <button className="btn" disabled={clearing} onClick={() => run(() => api.closeChrome(target.name))}
                            title={t('targets.closeChromeHint')}>
                        <Icon name="x"/> {t('targets.closeChrome')}
                    </button>
                ) : (
                    <button className="btn" disabled={!connected || clearing} onClick={() => run(() => api.openChrome(target.name))}
                            title={connected ? t('targets.openChromeHint') : t('targets.connectFirst')}>
                        <Icon name="globe"/> {t('targets.openChrome')}
                    </button>
                ))}
            </footer>

            {confirmClear && (
                <ConfirmDialog
                    title={t('targets.clearBrowserDataTitle')}
                    confirmLabel={t('targets.clearBrowserDataConfirm')}
                    confirmIcon="eraser"
                    onConfirm={clearBrowserData}
                    onCancel={() => setConfirmClear(false)}
                >
                    <p>{t('targets.clearBrowserDataBody', {profile: chromeProfile(target)})}</p>
                    {sharedWith.length > 0 && (
                        <p>{t('targets.clearBrowserDataShared', {targets: sharedWith.join(', ')})}</p>
                    )}
                    <p className="muted small">{t('targets.clearBrowserDataOtherEnvs', {profile: chromeProfile(target)})}</p>
                </ConfirmDialog>
            )}
        </article>
    )
}
