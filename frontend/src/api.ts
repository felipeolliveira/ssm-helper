// Typed wrappers around the Wails bindings. Go nil slices arrive as null,
// so they are normalized to empty arrays here.
import * as App from '../wailsjs/go/main/App'
import {env, main, settings} from '../wailsjs/go/models'
import {EventsOn} from '../wailsjs/runtime/runtime'

export type Profile = {
    name: string
    ssoUrl: string
    ssoRegion: string
    accountId: string
    roleName: string
}

export type Target = {
    name: string
    profile: string
    ssmTarget: string
    ssmRegion: string
    ssmDocument: string
    remoteHost: string
    remotePort: string
    localPort: string
    dnsHosts: string[]
    openUrls: string[]
    chromeProfile: string
}

export type Palette = {
    name: string
    // camelCase keys (bgDeep), applied as CSS variables (--bg-deep)
    colors: Record<string, string>
    builtIn: boolean
}

export type Appearance = {
    // Active palette: a built-in one (ssm-dark, ssm-light) or one of palettes
    palette: string
    // Palettes added in ssm-helper.json
    palettes: Palette[]
}

// An environment: env.json or env.<name>.json
export type Env = {
    path: string
    title: string
    profiles: Profile[]
    targets: Target[]
}

export type EnvState = {
    // Environment name; '' is env.json
    name: string
    file: string
    path: string
    env: Env | null
    problems: Message[]
    error: '' | 'notFound' | 'invalidJson' | 'read'
    errorDetail: string
}

export type EnvInfo = {
    name: string
    file: string
    activeTunnels: number
}

export type EnvList = {
    dir: string
    active: string
    envs: EnvInfo[]
}

export type NewEnvRequest = {
    name: string
    copyCurrent: boolean
    renameCurrentTo: string
}

// What the backend tells the user: a code translated by the i18n dictionaries
// ("messages"), its parameters and an optional raw technical detail.
export type Message = {
    code: string
    params?: Record<string, string>
    detail?: string
}

// Screen options stored in ssm-helper.json
export type Preferences = {
    // 'pt-BR', 'en' or '' (not chosen yet: follow Windows)
    language: string
    hideSensitive: boolean
    openChromeOnConnect: boolean
}

export type AppearanceState = {
    appearance: Appearance
    builtins: Palette[]
}

export type TunnelStatus = 'idle' | 'connecting' | 'login' | 'connected' | 'error'

export type TunnelState = {
    env: string
    target: string
    status: TunnelStatus
    message: Message
}

export type ProfileStatus = {
    name: string
    loggedIn: boolean
    expiresAt: string
}

export type LogEntry = {
    time: string
    source: string
    message: Message
}

export type SSOCode = {
    profile: string
    code: {userCode: string, verificationUri: string, verificationUriComplete: string}
}

function normalizeAppearance(a: any): Appearance {
    return {
        palette: a?.palette || 'ssm-dark',
        palettes: (a?.palettes ?? []).map((p: Palette) => ({...p, colors: {...(p.colors ?? {})}})),
    }
}

function normalizeEnv(c: any): Env {
    return {
        path: c.path ?? '',
        title: c.title ?? '',
        profiles: (c.profiles ?? []).map((p: Profile) => ({...p})),
        targets: (c.targets ?? []).map((t: Target) => ({
            ...t,
            dnsHosts: t.dnsHosts ?? [],
            openUrls: t.openUrls ?? [],
        })),
    }
}

function normalizeState(s: any): EnvState {
    return {
        name: s.name ?? '',
        file: s.file ?? '',
        path: s.path ?? '',
        env: s.env ? normalizeEnv(s.env) : null,
        problems: s.problems ?? [],
        error: s.error ?? '',
        errorDetail: s.errorDetail ?? '',
    }
}

export const api = {
    getEnv: async () => normalizeState(await App.GetEnv()),
    saveEnv: async (e: Env) => {
        const result = await App.SaveEnv(env.Env.createFrom(e))
        return {
            problems: (result.problems ?? []) as Message[],
            state: normalizeState(result.state),
        }
    },
    listEnvs: async (): Promise<EnvList> => {
        const list = await App.ListEnvs()
        return {dir: list.dir ?? '', active: list.active ?? '', envs: (list.envs ?? []) as EnvInfo[]}
    },
    selectEnv: async (name: string) => normalizeState(await App.SelectEnv(name)),
    createEnv: async (req: NewEnvRequest) => normalizeState(await App.CreateEnv(main.NewEnvRequest.createFrom(req))),
    openAppFolder: () => App.OpenAppFolder(),
    getAppearance: async (): Promise<AppearanceState> => {
        const state = await App.GetAppearance()
        return {
            appearance: normalizeAppearance(state.appearance),
            builtins: normalizeAppearance({palettes: state.builtins}).palettes,
        }
    },
    saveAppearance: async (a: Appearance) => ((await App.SaveAppearance(settings.Appearance.createFrom(a))) ?? []) as Message[],
    getPreferences: async (): Promise<Preferences> => {
        const p = await App.GetPreferences()
        return {language: p.language ?? '', hideSensitive: !!p.hideSensitive, openChromeOnConnect: !!p.openChromeOnConnect}
    },
    setLanguage: (lang: string) => App.SetLanguage(lang),
    setHideSensitive: (hide: boolean) => App.SetHideSensitive(hide),
    setOpenChromeOnConnect: (open: boolean) => App.SetOpenChromeOnConnect(open),
    startupMessages: async () => ((await App.StartupMessages()) ?? []) as LogEntry[],
    profileStatuses: async () => ((await App.ProfileStatuses()) ?? []) as ProfileStatus[],
    login: (profile: string) => App.Login(profile),
    cancelLogin: (profile: string) => App.CancelLogin(profile),
    tunnelStates: async () => ((await App.TunnelStates()) ?? []) as TunnelState[],
    connect: (target: string, openChrome: boolean) => App.Connect(target, openChrome),
    disconnect: (envName: string, target: string) => App.Disconnect(envName, target),
    openChrome: (target: string) => App.OpenChrome(target),
}

// Tunnels are identified by environment and target, since environments may reuse target names
export function tunnelKey(envName: string, target: string) {
    return `${envName}/${target}`
}

// Display name of an environment: env.cliente-qa.json is CLIENTE-QA, env.json is DEFAULT
export function envLabel(name: string) {
    return name ? name.toUpperCase() : 'DEFAULT'
}

export const events = {
    onTunnelState: (cb: (s: TunnelState) => void) => EventsOn('tunnel:state', cb),
    onLog: (cb: (e: LogEntry) => void) => EventsOn('log', cb),
    onSSOCode: (cb: (c: SSOCode) => void) => EventsOn('sso:code', cb),
}

// Binding errors arrive as the JSON of a Message (internal/msg); anything
// else becomes UNEXPECTED_ERROR with the text as detail.
export function toMessage(err: unknown): Message {
    const text = typeof err === 'string' ? err : err instanceof Error ? err.message : String(err)
    try {
        const parsed = JSON.parse(text)
        if (parsed && typeof parsed.code === 'string') return parsed as Message
    } catch {
        // not a Message
    }
    return {code: 'UNEXPECTED_ERROR', detail: text}
}

// Message built in the frontend, for checks done before calling the backend
export function localMessage(code: string, params?: Record<string, string | number>): Message {
    return {code, params: params && Object.fromEntries(Object.entries(params).map(([k, v]) => [k, String(v)]))}
}

export function emptyProfile(): Profile {
    return {name: '', ssoUrl: '', ssoRegion: 'us-east-1', accountId: '', roleName: ''}
}

export function emptyTarget(profile = ''): Target {
    return {
        name: '',
        profile,
        ssmTarget: '',
        ssmRegion: '',
        ssmDocument: 'AWS-StartPortForwardingSessionToRemoteHost',
        remoteHost: '',
        remotePort: '',
        localPort: '',
        dnsHosts: [],
        openUrls: [],
        chromeProfile: '',
    }
}

export function emptyEnv(path: string): Env {
    return {path, title: 'SSM Helper', profiles: [emptyProfile()], targets: []}
}
