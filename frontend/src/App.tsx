import {useCallback, useEffect, useMemo, useState} from 'react'
import {
    api,
    Appearance,
    AppearanceState,
    EnvList,
    EnvState,
    errorMessage,
    events,
    LogEntry,
    ProfileStatus,
    SSOCode,
    TunnelState,
    tunnelKey,
} from './api'
import {activePalette, applyPalette} from './theme'
import TargetsView from './views/TargetsView'
import ProfilesView from './views/ProfilesView'
import EnvView from './views/EnvView'
import AppearanceView from './views/AppearanceView'
import LogPanel from './components/LogPanel'
import SSOBanner from './components/SSOBanner'
import EnvSwitcher from './components/EnvSwitcher'
import NewEnvDialog from './components/NewEnvDialog'
import {Icon, IconName} from './components/Icon'
import {usePrivacy} from './privacy'

type View = 'targets' | 'profiles' | 'env' | 'appearance'

const MAX_LOGS = 500

export default function App() {
    const [envState, setEnvState] = useState<EnvState | null>(null)
    const [envList, setEnvList] = useState<EnvList | null>(null)
    const [view, setView] = useState<View>('targets')
    // All tunnels, of every environment, keyed by tunnelKey(env, target)
    const [tunnels, setTunnels] = useState<Record<string, TunnelState>>({})
    const [profiles, setProfiles] = useState<ProfileStatus[]>([])
    const [logs, setLogs] = useState<LogEntry[]>([])
    const [ssoCodes, setSSOCodes] = useState<Record<string, SSOCode>>({})
    const [logOpen, setLogOpen] = useState(false)
    const [autoChrome, setAutoChrome] = useState(true)
    const [loadError, setLoadError] = useState('')
    const [appearanceState, setAppearanceState] = useState<AppearanceState | null>(null)
    // Unsaved palette edits from the appearance screen, shown live
    const [previewAppearance, setPreviewAppearance] = useState<Appearance | null>(null)
    const [newEnvOpen, setNewEnvOpen] = useState(false)
    const privacy = usePrivacy()

    const currentEnv = envState?.env ?? null
    const envUsable = !!currentEnv && !!envState && !envState.error && envState.problems.length === 0
    const appearance = previewAppearance ?? appearanceState?.appearance ?? null

    useEffect(() => {
        const palette = appearance && appearanceState && activePalette(appearance, appearanceState.builtins)
        if (palette) applyPalette(palette)
    }, [appearance, appearanceState])

    const addLogs = useCallback((entries: LogEntry[]) => {
        setLogs(prev => [...prev, ...entries].slice(-MAX_LOGS))
    }, [])

    const refreshProfiles = useCallback(async () => {
        try {
            setProfiles(await api.profileStatuses())
        } catch {
            setProfiles([])
        }
    }, [])

    const refreshEnvList = useCallback(async () => {
        try {
            setEnvList(await api.listEnvs())
        } catch (err) {
            setLoadError(errorMessage(err))
        }
    }, [])

    const applyEnvState = useCallback((state: EnvState) => {
        setEnvState(state)
        if (state.error || state.problems.length > 0) setView('env')
        refreshProfiles()
        refreshEnvList()
    }, [refreshProfiles, refreshEnvList])

    const reloadEnv = useCallback(async () => {
        try {
            applyEnvState(await api.getEnv())
            setLoadError('')
        } catch (err) {
            setLoadError(errorMessage(err))
        }
    }, [applyEnvState])

    const selectEnv = async (name: string) => {
        try {
            const state = await api.selectEnv(name)
            applyEnvState(state)
            if (!state.error && state.problems.length === 0 && view === 'env') setView('targets')
            setLoadError('')
        } catch (err) {
            setLoadError(errorMessage(err))
        }
    }

    useEffect(() => {
        reloadEnv()
        api.getAppearance().then(setAppearanceState).catch(err => setLoadError(errorMessage(err)))
        api.getPreferences().then(p => setAutoChrome(p.openChromeOnConnect)).catch(() => {})
        api.startupMessages().then(entries => {
            addLogs(entries)
            if (entries.length > 0) setLogOpen(true)
        })
        api.tunnelStates().then(states => {
            setTunnels(Object.fromEntries(states.map(s => [tunnelKey(s.env, s.target), s])))
        })
        const offState = events.onTunnelState(s => {
            setTunnels(prev => ({...prev, [tunnelKey(s.env, s.target)]: s}))
            if (s.status === 'connected' || s.status === 'error') refreshProfiles()
            if (s.status === 'connected' || s.status === 'idle' || s.status === 'error') refreshEnvList()
        })
        const offLog = events.onLog(entry => addLogs([entry]))
        const offCode = events.onSSOCode(code => {
            setSSOCodes(prev => {
                const next = {...prev}
                if (code.code?.userCode) next[code.profile] = code
                else delete next[code.profile]
                return next
            })
            if (!code.code?.userCode) refreshProfiles()
        })
        return () => {
            offState()
            offLog()
            offCode()
        }
    }, [reloadEnv, refreshProfiles, refreshEnvList, addLogs])

    const toggleAutoChrome = (value: boolean) => {
        setAutoChrome(value)
        api.setOpenChromeOnConnect(value).catch(err => setLoadError(errorMessage(err)))
    }

    const activeName = envState?.name ?? ''
    const activeCount = useMemo(
        () => Object.values(tunnels).filter(t => t.env === activeName && t.status === 'connected').length,
        [tunnels, activeName],
    )

    const nav: {id: View, label: string, icon: IconName, disabled?: boolean}[] = [
        {id: 'targets', label: 'Targets', icon: 'plug', disabled: !envUsable},
        {id: 'profiles', label: 'Perfis AWS', icon: 'user', disabled: !envUsable},
        {id: 'env', label: 'Ambiente', icon: 'settings'},
        {id: 'appearance', label: 'Aparência', icon: 'palette', disabled: !appearanceState},
    ]

    return (
        <div className="shell">
            <aside className="sidebar">
                <div className="brand">
                    <span className="brand-mark"><Icon name="tunnel"/></span>
                    <div className="brand-text">
                        {/* The window title already says SSM Helper, so the sidebar shows the environment title */}
                        <div className="brand-title" title={currentEnv?.title}>{currentEnv?.title || 'SSM Helper'}</div>
                    </div>
                    <button className={`btn ghost icon-only privacy-toggle ${privacy.hidden ? 'on' : ''}`}
                            onClick={privacy.toggle} aria-pressed={privacy.hidden}
                            title={privacy.hidden ? 'Mostrar dados sensíveis' : 'Ocultar dados sensíveis (contas, hosts, domínios)'}>
                        <Icon name={privacy.hidden ? 'eyeOff' : 'eye'} size={18}/>
                    </button>
                </div>
                <nav>
                    {nav.map(item => (
                        <button
                            key={item.id}
                            className={`nav-item ${view === item.id ? 'active' : ''}`}
                            disabled={item.disabled}
                            onClick={() => setView(item.id)}
                        >
                            <Icon name={item.icon}/>
                            <span>{item.label}</span>
                            {item.id === 'targets' && activeCount > 0 && (
                                <span className="nav-badge">{activeCount}</span>
                            )}
                            {item.id === 'env' && envState && (envState.error || envState.problems.length > 0) && (
                                <span className="nav-badge warn">!</span>
                            )}
                        </button>
                    ))}
                </nav>
                <div className="sidebar-footer">
                    <EnvSwitcher list={envList} onSelect={selectEnv} onNew={() => setNewEnvOpen(true)}/>
                    <button className={`nav-item ${logOpen ? 'active' : ''}`} onClick={() => setLogOpen(o => !o)}>
                        <Icon name="terminal"/>
                        <span>Log</span>
                        {logs.length > 0 && <span className="nav-count">{logs.length}</span>}
                    </button>
                </div>
            </aside>

            <main className="main">
                <SSOBanner codes={Object.values(ssoCodes)} onCancel={profile => api.cancelLogin(profile)}/>
                {loadError && <div className="alert error">{loadError}</div>}
                <div className="content">
                    {view === 'appearance' && appearanceState ? (
                        <AppearanceView
                            saved={appearanceState.appearance}
                            builtins={appearanceState.builtins}
                            onPreview={setPreviewAppearance}
                            onSaved={a => setAppearanceState(s => (s ? {...s, appearance: a} : s))}
                        />
                    ) : !envState ? (
                        <div className="empty">Carregando ambiente...</div>
                    ) : view === 'targets' && envUsable ? (
                        <TargetsView
                            envName={activeName}
                            env={currentEnv!}
                            tunnels={tunnels}
                            profiles={profiles}
                            autoChrome={autoChrome}
                            onAutoChromeChange={toggleAutoChrome}
                        />
                    ) : view === 'profiles' && envUsable ? (
                        <ProfilesView env={currentEnv!} statuses={profiles} onRefresh={refreshProfiles}/>
                    ) : (
                        <EnvView
                            key={activeName}
                            state={envState}
                            hasActiveTunnels={activeCount > 0}
                            onSaved={applyEnvState}
                            onReload={reloadEnv}
                        />
                    )}
                </div>
                <LogPanel open={logOpen} logs={logs} onClose={() => setLogOpen(false)} onClear={() => setLogs([])}/>
            </main>

            {newEnvOpen && envList && (
                <NewEnvDialog
                    list={envList}
                    current={envState}
                    onClose={() => setNewEnvOpen(false)}
                    onCreated={state => {
                        setNewEnvOpen(false)
                        applyEnvState(state)
                        setView('env')
                    }}
                />
            )}
        </div>
    )
}
