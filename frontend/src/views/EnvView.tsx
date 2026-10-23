import {ChangeEvent, useEffect, useMemo, useState} from 'react'
import {api, Env, EnvState, emptyEnv, emptyProfile, emptyTarget, localMessage, Message, Profile, Target, toMessage} from '../api'
import {Icon} from '../components/Icon'
import {Field, MasterDetail} from '../components/Form'
import {Secret, usePrivacy, useSecretInput, useSecretText} from '../privacy'
import {useI18n} from '../i18n'

type Tab = 'general' | 'profiles' | 'targets'

const NAME_PATTERN = /^[A-Za-z0-9._-]+$/

function splitList(value: string): string[] {
    return value.split(/\s+/).filter(Boolean)
}

// Checks done before calling the backend, which validates everything again.
function localProblems(c: Env): Message[] {
    const problems: Message[] = []
    const check = (kind: 'PROFILE' | 'TARGET', names: string[]) => {
        const seen = new Set<string>()
        names.forEach((name, i) => {
            if (!name) problems.push(localMessage(`LOCAL_${kind}_UNNAMED`, {index: i + 1}))
            else if (seen.has(name)) problems.push(localMessage(`LOCAL_${kind}_DUPLICATE`, {name}))
            seen.add(name)
        })
    }
    check('PROFILE', c.profiles.map(p => p.name.trim()))
    check('TARGET', c.targets.map(t => t.name.trim()))
    return problems
}

export default function EnvView({state, hasActiveTunnels, onSaved, onReload}: {
    state: EnvState
    hasActiveTunnels: boolean
    onSaved: (state: EnvState) => void
    onReload: () => void
}) {
    const {t, tm} = useI18n()
    const {hidden} = usePrivacy()
    // Problems can quote invalid hosts and URLs
    const secretText = useSecretText()
    const [draft, setDraft] = useState<Env | null>(state.env)
    const [tab, setTab] = useState<Tab>('targets')
    const [selectedProfile, setSelectedProfile] = useState(0)
    const [selectedTarget, setSelectedTarget] = useState(0)
    const [problems, setProblems] = useState<Message[]>(state.problems)
    const [saving, setSaving] = useState(false)
    const [saveError, setSaveError] = useState<Message | null>(null)
    const [savedNotice, setSavedNotice] = useState(false)

    useEffect(() => {
        setDraft(state.env)
        setProblems(state.problems)
    }, [state])

    const dirty = useMemo(
        () => JSON.stringify(draft) !== JSON.stringify(state.env),
        [draft, state.env],
    )

    const update = (fn: (c: Env) => Env) => {
        setSavedNotice(false)
        setDraft(c => (c ? fn(c) : c))
    }

    const save = async () => {
        if (!draft) return
        const local = localProblems(draft)
        if (local.length > 0) {
            setProblems(local)
            return
        }
        setSaving(true)
        setSaveError(null)
        try {
            const result = await api.saveEnv(draft)
            if (result.problems.length > 0) {
                setProblems(result.problems)
            } else {
                setProblems([])
                setSavedNotice(true)
                onSaved(result.state)
            }
        } catch (err) {
            setSaveError(toMessage(err))
        } finally {
            setSaving(false)
        }
    }

    const discard = () => {
        setDraft(state.env)
        setProblems(state.problems)
        setSaveError(null)
    }

    const startFromScratch = () => {
        setDraft(emptyEnv(state.path))
        setTab('profiles')
        setSelectedProfile(0)
    }

    return (
        <div className="page">
            <header className="page-header">
                <div className="spacer">
                    <h1>{t('env.title')}</h1>
                    <p className="muted small mono" title={hidden ? undefined : state.path}><Secret value={state.path}/></p>
                </div>
                <button className="btn ghost" onClick={() => api.openAppFolder()}><Icon name="folder"/> {t('env.openFolder')}</button>
                <button className="btn ghost" onClick={onReload} title={t('env.reloadHint', {file: state.file})}><Icon name="refresh"/> {t('common.reload')}</button>
                {draft && (
                    <>
                        <button className="btn" onClick={discard} disabled={!dirty || saving}>{t('common.discard')}</button>
                        <button className="btn primary" onClick={save} disabled={(!dirty && state.env !== null) || saving}>
                            <Icon name="save"/> {saving ? t('common.saving') : t('common.save')}
                        </button>
                    </>
                )}
            </header>

            {state.error === 'notFound' && !draft && (
                <div className="card empty-state">
                    <Icon name="settings" size={28}/>
                    <h2>{t('env.notFoundTitle')}</h2>
                    <p className="muted">{t('env.notFoundText', {file: state.file})}</p>
                    <button className="btn primary" onClick={startFromScratch}><Icon name="plus"/> {t('env.create')}</button>
                </div>
            )}

            {state.error === 'invalidJson' && !draft && (
                <div className="card empty-state">
                    <Icon name="alert" size={28}/>
                    <h2>{t('env.invalidJsonTitle')}</h2>
                    <pre className="error-detail">{state.errorDetail}</pre>
                    <p className="muted">{t('env.invalidJsonText', {file: state.file})}</p>
                    <div className="row">
                        <button className="btn" onClick={onReload}><Icon name="refresh"/> {t('common.reload')}</button>
                        <button className="btn primary" onClick={startFromScratch}>{t('env.startOver')}</button>
                    </div>
                </div>
            )}

            {state.error === 'read' && (
                <div className="alert error">{t('env.readError', {file: state.file})} {state.errorDetail}</div>
            )}

            {problems.length > 0 && (
                <div className="alert warn">
                    <Icon name="alert" size={18}/>
                    <div>
                        <strong>{t('common.fixBeforeUse')}</strong>
                        <ul className={secretText}>{problems.map((p, i) => <li key={i}><code>{tm(p)}</code></li>)}</ul>
                    </div>
                </div>
            )}
            {saveError && <div className={`alert error ${secretText}`}>{tm(saveError)}</div>}
            {savedNotice && !dirty && <div className="alert success"><Icon name="check"/> {t('env.saved')}</div>}
            {hasActiveTunnels && dirty && (
                <div className="alert info">{t('env.tunnelsKeepOld')}</div>
            )}

            {draft && (
                <>
                    <div className="tabs">
                        <button className={tab === 'general' ? 'active' : ''} onClick={() => setTab('general')}>{t('env.tabs.general')}</button>
                        <button className={tab === 'profiles' ? 'active' : ''} onClick={() => setTab('profiles')}>
                            {t('env.tabs.profiles')} <span className="count">{draft.profiles.length}</span>
                        </button>
                        <button className={tab === 'targets' ? 'active' : ''} onClick={() => setTab('targets')}>
                            {t('env.tabs.targets')} <span className="count">{draft.targets.length}</span>
                        </button>
                    </div>

                    {tab === 'general' && (
                        <div className="card form-card">
                            <Field label={t('env.general.title')} hint={t('env.general.titleHint')}>
                                <input value={draft.title} onChange={e => update(c => ({...c, title: e.target.value}))}/>
                            </Field>
                        </div>
                    )}

                    {tab === 'profiles' && (
                        <MasterDetail
                            items={draft.profiles.map(p => ({name: p.name, sub: <Secret value={p.accountId} keepLast={4}/>}))}
                            selected={selectedProfile}
                            onSelect={setSelectedProfile}
                            addLabel={t('env.profile.new')}
                            onAdd={() => {
                                update(c => ({...c, profiles: [...c.profiles, emptyProfile()]}))
                                setSelectedProfile(draft.profiles.length)
                            }}
                            onRemove={index => {
                                update(c => ({...c, profiles: c.profiles.filter((_, i) => i !== index)}))
                                setSelectedProfile(Math.max(0, index - 1))
                            }}
                            removeWarning={index => {
                                const name = draft.profiles[index]?.name
                                const users = draft.targets.filter(t => t.profile === name).map(t => t.name)
                                return users.length > 0 ? t('env.profile.usedBy', {targets: users.join(', ')}) : ''
                            }}
                            render={index => (
                                <ProfileForm
                                    profile={draft.profiles[index]}
                                    onChange={(next, previousName) => update(c => ({
                                        ...c,
                                        profiles: c.profiles.map((p, i) => (i === index ? next : p)),
                                        // Renaming a profile keeps the targets that use it
                                        targets: previousName !== next.name
                                            ? c.targets.map(t => (t.profile === previousName ? {...t, profile: next.name} : t))
                                            : c.targets,
                                    }))}
                                />
                            )}
                        />
                    )}

                    {tab === 'targets' && (
                        <MasterDetail
                            items={draft.targets.map(t => ({name: t.name, sub: t.profile}))}
                            selected={selectedTarget}
                            onSelect={setSelectedTarget}
                            addLabel={t('env.target.new')}
                            onAdd={() => {
                                update(c => ({...c, targets: [...c.targets, emptyTarget(c.profiles[0]?.name ?? '')]}))
                                setSelectedTarget(draft.targets.length)
                            }}
                            onRemove={index => {
                                update(c => ({...c, targets: c.targets.filter((_, i) => i !== index)}))
                                setSelectedTarget(Math.max(0, index - 1))
                            }}
                            render={index => (
                                <TargetForm
                                    key={index}
                                    target={draft.targets[index]}
                                    profiles={draft.profiles}
                                    onChange={next => update(c => ({
                                        ...c,
                                        targets: c.targets.map((t, i) => (i === index ? next : t)),
                                    }))}
                                />
                            )}
                        />
                    )}
                </>
            )}
        </div>
    )
}

// Field checks shown while typing
function useFieldChecks() {
    const {t} = useI18n()
    return {
        nameError: (name: string) => (!name ? t('common.required') : NAME_PATTERN.test(name) ? '' : t('common.nameRule')),
        required: (value: string) => (value.trim() ? '' : t('common.required')),
    }
}

function ProfileForm({profile, onChange}: {profile: Profile, onChange: (p: Profile, previousName: string) => void}) {
    const {t} = useI18n()
    const {nameError, required} = useFieldChecks()
    const secret = useSecretInput()
    const set = (field: keyof Profile) => (e: ChangeEvent<HTMLInputElement>) =>
        onChange({...profile, [field]: e.target.value}, profile.name)
    return (
        <div className="form-grid">
            <Field label={t('env.profile.name')} error={nameError(profile.name)} hint={t('env.profile.nameHint')}>
                <input value={profile.name} onChange={set('name')} placeholder="qa-admin"/>
            </Field>
            <Field label={t('env.profile.ssoRegion')} error={required(profile.ssoRegion)} hint={t('env.profile.ssoRegionHint')}>
                <input value={profile.ssoRegion} onChange={set('ssoRegion')} placeholder="us-east-1"/>
            </Field>
            <Field label={t('env.profile.ssoUrl')} error={required(profile.ssoUrl)} wide>
                <input value={profile.ssoUrl} {...secret(profile.ssoUrl)} onChange={set('ssoUrl')}
                       placeholder="https://identitycenter.amazonaws.com/ssoins-..."/>
            </Field>
            <Field label={t('env.profile.accountId')} error={required(profile.accountId)}>
                <input value={profile.accountId} {...secret(profile.accountId)} onChange={set('accountId')} placeholder="111111111111"/>
            </Field>
            <Field label={t('env.profile.roleName')} error={required(profile.roleName)}>
                <input value={profile.roleName} onChange={set('roleName')} placeholder="AdministratorAccess"/>
            </Field>
        </div>
    )
}

function TargetForm({target, profiles, onChange}: {
    target: Target
    profiles: Profile[]
    onChange: (t: Target) => void
}) {
    const {t} = useI18n()
    const {nameError, required} = useFieldChecks()
    const secret = useSecretInput()
    const set = (field: keyof Target) => (e: ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
        onChange({...target, [field]: e.target.value})
    const profileRegion = profiles.find(p => p.name === target.profile)?.ssoRegion
    // Lists are edited as text, so trailing spaces are kept while typing
    const [dnsText, setDnsText] = useState(target.dnsHosts.join(' '))
    const [urlText, setUrlText] = useState(target.openUrls.join(' '))

    useEffect(() => {
        if (splitList(dnsText).join(' ') !== target.dnsHosts.join(' ')) setDnsText(target.dnsHosts.join(' '))
        if (splitList(urlText).join(' ') !== target.openUrls.join(' ')) setUrlText(target.openUrls.join(' '))
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [target.dnsHosts, target.openUrls])

    return (
        <div className="form-grid">
            <h3 className="form-section wide">{t('env.target.connection')}</h3>
            <Field label={t('env.target.name')} error={nameError(target.name)}>
                <input value={target.name} onChange={set('name')} placeholder="qa-web"/>
            </Field>
            <Field label={t('env.target.profile')} error={required(target.profile)}>
                <select value={target.profile} onChange={set('profile')}>
                    <option value="">{t('env.target.selectProfile')}</option>
                    {profiles.filter(p => p.name).map(p => <option key={p.name} value={p.name}>{p.name}</option>)}
                </select>
            </Field>
            <Field label={t('env.target.ssmTarget')} error={required(target.ssmTarget)} hint={t('env.target.ssmTargetHint')}>
                <input value={target.ssmTarget} {...secret(target.ssmTarget)} onChange={set('ssmTarget')} placeholder="i-0123456789abcdef0"/>
            </Field>
            <Field label={t('env.target.region')}
                   hint={profileRegion ? t('env.target.regionHintWithValue', {region: profileRegion}) : t('env.target.regionHint')}>
                <input value={target.ssmRegion} onChange={set('ssmRegion')} placeholder={profileRegion || 'us-east-1'}/>
            </Field>
            <Field label={t('env.target.document')} error={required(target.ssmDocument)} wide>
                <input value={target.ssmDocument} onChange={set('ssmDocument')}/>
            </Field>
            <Field label={t('env.target.remoteHost')} error={required(target.remoteHost)} hint={t('env.target.remoteHostHint')} wide>
                <input value={target.remoteHost} {...secret(target.remoteHost)} onChange={set('remoteHost')} placeholder="internal-meu-alb.us-east-1.elb.amazonaws.com"/>
            </Field>
            <Field label={t('env.target.remotePort')} error={required(target.remotePort)}>
                <input value={target.remotePort} onChange={set('remotePort')} inputMode="numeric" placeholder="443"/>
            </Field>
            <Field label={t('env.target.localPort')} error={required(target.localPort)} hint={t('env.target.localPortHint')}>
                <input value={target.localPort} onChange={set('localPort')} inputMode="numeric" placeholder="8443"/>
            </Field>

            <h3 className="form-section wide">{t('env.target.browser')}</h3>
            <Field label={t('env.target.dnsHosts')} wide hint={t('env.target.dnsHostsHint')}>
                <input value={dnsText} {...secret(dnsText)} placeholder="qa.example.com api.qa.example.com"
                       onChange={e => {
                           setDnsText(e.target.value)
                           onChange({...target, dnsHosts: splitList(e.target.value)})
                       }}/>
            </Field>
            <Field label={t('env.target.openUrls')} wide hint={t('env.target.openUrlsHint')}>
                <input value={urlText} {...secret(urlText)} placeholder="qa.example.com"
                       onChange={e => {
                           setUrlText(e.target.value)
                           onChange({...target, openUrls: splitList(e.target.value)})
                       }}/>
            </Field>
            <Field label={t('env.target.chromeProfile')}
                   error={target.chromeProfile && !NAME_PATTERN.test(target.chromeProfile) ? t('common.nameRule') : ''}
                   hint={t('env.target.chromeProfileHint')}>
                <input value={target.chromeProfile} onChange={set('chromeProfile')} placeholder={target.name || 'qa'}/>
            </Field>
        </div>
    )
}
