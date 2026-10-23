import {ChangeEvent, useEffect, useMemo, useState} from 'react'
import {api, Env, EnvState, emptyEnv, emptyProfile, emptyTarget, errorMessage, Profile, Target} from '../api'
import {Icon} from '../components/Icon'
import {Field, MasterDetail} from '../components/Form'

type Tab = 'general' | 'profiles' | 'targets'

const NAME_PATTERN = /^[A-Za-z0-9._-]+$/

function splitList(value: string): string[] {
    return value.split(/\s+/).filter(Boolean)
}

// Checks done before calling the backend, which validates everything again.
function localProblems(c: Env): string[] {
    const problems: string[] = []
    const check = (kind: string, names: string[]) => {
        const seen = new Set<string>()
        names.forEach((name, i) => {
            if (!name) problems.push(`${kind} #${i + 1} está sem nome`)
            else if (seen.has(name)) problems.push(`${kind} '${name}' está duplicado`)
            seen.add(name)
        })
    }
    check('Perfil', c.profiles.map(p => p.name.trim()))
    check('Target', c.targets.map(t => t.name.trim()))
    return problems
}

export default function EnvView({state, hasActiveTunnels, onSaved, onReload}: {
    state: EnvState
    hasActiveTunnels: boolean
    onSaved: (state: EnvState) => void
    onReload: () => void
}) {
    const [draft, setDraft] = useState<Env | null>(state.env)
    const [tab, setTab] = useState<Tab>('targets')
    const [selectedProfile, setSelectedProfile] = useState(0)
    const [selectedTarget, setSelectedTarget] = useState(0)
    const [problems, setProblems] = useState<string[]>(state.problems)
    const [saving, setSaving] = useState(false)
    const [saveError, setSaveError] = useState('')
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
        setSaveError('')
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
            setSaveError(errorMessage(err))
        } finally {
            setSaving(false)
        }
    }

    const discard = () => {
        setDraft(state.env)
        setProblems(state.problems)
        setSaveError('')
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
                    <h1>Ambiente</h1>
                    <p className="muted small mono" title={state.path}>{state.path}</p>
                </div>
                <button className="btn ghost" onClick={() => api.openAppFolder()}><Icon name="folder"/> Abrir pasta</button>
                <button className="btn ghost" onClick={onReload} title={`Lê de novo o ${state.file} do disco`}><Icon name="refresh"/> Recarregar</button>
                {draft && (
                    <>
                        <button className="btn" onClick={discard} disabled={!dirty || saving}>Descartar</button>
                        <button className="btn primary" onClick={save} disabled={(!dirty && state.env !== null) || saving}>
                            <Icon name="save"/> {saving ? 'Salvando...' : 'Salvar'}
                        </button>
                    </>
                )}
            </header>

            {state.error === 'notFound' && !draft && (
                <div className="card empty-state">
                    <Icon name="settings" size={28}/>
                    <h2>Nenhum ambiente encontrado</h2>
                    <p className="muted">O <code>{state.file}</code> não existe na pasta do programa. Crie o ambiente por aqui.</p>
                    <button className="btn primary" onClick={startFromScratch}><Icon name="plus"/> Criar ambiente</button>
                </div>
            )}

            {state.error === 'invalidJson' && !draft && (
                <div className="card empty-state">
                    <Icon name="alert" size={28}/>
                    <h2>O arquivo não é um JSON válido</h2>
                    <pre className="error-detail">{state.errorDetail}</pre>
                    <p className="muted">Corrija o <code>{state.file}</code> e recarregue, ou recomece pela tela; o arquivo atual
                        fica salvo como <code>{state.file}.bak</code>.</p>
                    <div className="row">
                        <button className="btn" onClick={onReload}><Icon name="refresh"/> Recarregar</button>
                        <button className="btn primary" onClick={startFromScratch}>Recomeçar pela tela</button>
                    </div>
                </div>
            )}

            {state.error === 'read' && (
                <div className="alert error">Não foi possível ler o {state.file}: {state.errorDetail}</div>
            )}

            {problems.length > 0 && (
                <div className="alert warn">
                    <Icon name="alert" size={18}/>
                    <div>
                        <strong>Corrija os campos abaixo antes de usar:</strong>
                        <ul>{problems.map(p => <li key={p}><code>{p}</code></li>)}</ul>
                    </div>
                </div>
            )}
            {saveError && <div className="alert error">{saveError}</div>}
            {savedNotice && !dirty && <div className="alert success"><Icon name="check"/> Ambiente salvo.</div>}
            {hasActiveTunnels && dirty && (
                <div className="alert info">Os túneis abertos continuam com as definições anteriores até serem reconectados.</div>
            )}

            {draft && (
                <>
                    <div className="tabs">
                        <button className={tab === 'general' ? 'active' : ''} onClick={() => setTab('general')}>Geral</button>
                        <button className={tab === 'profiles' ? 'active' : ''} onClick={() => setTab('profiles')}>
                            Perfis <span className="count">{draft.profiles.length}</span>
                        </button>
                        <button className={tab === 'targets' ? 'active' : ''} onClick={() => setTab('targets')}>
                            Targets <span className="count">{draft.targets.length}</span>
                        </button>
                    </div>

                    {tab === 'general' && (
                        <div className="card form-card">
                            <Field label="Título" hint="Aparece no topo da barra lateral.">
                                <input value={draft.title} onChange={e => update(c => ({...c, title: e.target.value}))}/>
                            </Field>
                        </div>
                    )}

                    {tab === 'profiles' && (
                        <MasterDetail
                            items={draft.profiles.map(p => ({name: p.name, sub: p.accountId}))}
                            selected={selectedProfile}
                            onSelect={setSelectedProfile}
                            addLabel="Novo perfil"
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
                                return users.length > 0 ? `Usado por: ${users.join(', ')}` : ''
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
                            addLabel="Novo target"
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

function nameError(name: string) {
    if (!name) return 'Obrigatório'
    if (!NAME_PATTERN.test(name)) return "Use apenas letras, números, '.', '_' ou '-'"
    return ''
}

function required(value: string) {
    return value.trim() ? '' : 'Obrigatório'
}

function ProfileForm({profile, onChange}: {profile: Profile, onChange: (p: Profile, previousName: string) => void}) {
    const set = (field: keyof Profile) => (e: ChangeEvent<HTMLInputElement>) =>
        onChange({...profile, [field]: e.target.value}, profile.name)
    return (
        <div className="form-grid">
            <Field label="Nome" error={nameError(profile.name)} hint="Usado para escolher o perfil nos targets.">
                <input value={profile.name} onChange={set('name')} placeholder="qa-admin"/>
            </Field>
            <Field label="Região do SSO" error={required(profile.ssoRegion)} hint="Também é a região padrão dos targets.">
                <input value={profile.ssoRegion} onChange={set('ssoRegion')} placeholder="us-east-1"/>
            </Field>
            <Field label="URL de início do SSO" error={required(profile.ssoUrl)} wide>
                <input value={profile.ssoUrl} onChange={set('ssoUrl')}
                       placeholder="https://identitycenter.amazonaws.com/ssoins-..."/>
            </Field>
            <Field label="ID da conta" error={required(profile.accountId)}>
                <input value={profile.accountId} onChange={set('accountId')} placeholder="111111111111"/>
            </Field>
            <Field label="Role (permission set)" error={required(profile.roleName)}>
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
            <h3 className="form-section wide">Conexão</h3>
            <Field label="Nome" error={nameError(target.name)}>
                <input value={target.name} onChange={set('name')} placeholder="qa-web"/>
            </Field>
            <Field label="Perfil AWS" error={required(target.profile)}>
                <select value={target.profile} onChange={set('profile')}>
                    <option value="">Selecione...</option>
                    {profiles.filter(p => p.name).map(p => <option key={p.name} value={p.name}>{p.name}</option>)}
                </select>
            </Field>
            <Field label="Instância (ssmTarget)" error={required(target.ssmTarget)} hint="EC2 com o SSM Agent, usada como ponte.">
                <input value={target.ssmTarget} onChange={set('ssmTarget')} placeholder="i-0123456789abcdef0"/>
            </Field>
            <Field label="Região" hint={profileRegion ? `Vazio usa a região do perfil (${profileRegion}).` : 'Vazio usa a região do perfil.'}>
                <input value={target.ssmRegion} onChange={set('ssmRegion')} placeholder={profileRegion || 'us-east-1'}/>
            </Field>
            <Field label="Documento SSM" error={required(target.ssmDocument)} wide>
                <input value={target.ssmDocument} onChange={set('ssmDocument')}/>
            </Field>
            <Field label="Host remoto" error={required(target.remoteHost)} hint="Acessível a partir da instância." wide>
                <input value={target.remoteHost} onChange={set('remoteHost')} placeholder="internal-meu-alb.us-east-1.elb.amazonaws.com"/>
            </Field>
            <Field label="Porta remota" error={required(target.remotePort)}>
                <input value={target.remotePort} onChange={set('remotePort')} inputMode="numeric" placeholder="443"/>
            </Field>
            <Field label="Porta local" error={required(target.localPort)} hint="Porta exposta na sua máquina.">
                <input value={target.localPort} onChange={set('localPort')} inputMode="numeric" placeholder="8443"/>
            </Field>

            <h3 className="form-section wide">Navegador (opcional)</h3>
            <Field label="Domínios no Chrome (dnsHosts)" wide
                   hint="Separados por espaço. O Chrome aberto pelo app resolve esses domínios para o túnel. Aceita curinga, como *.qa.example.com.">
                <input value={dnsText} placeholder="qa.example.com api.qa.example.com"
                       onChange={e => {
                           setDnsText(e.target.value)
                           onChange({...target, dnsHosts: splitList(e.target.value)})
                       }}/>
            </Field>
            <Field label="URLs abertas ao conectar (openUrls)" wide hint="Separadas por espaço. Sem http(s)://, usa https://.">
                <input value={urlText} placeholder="qa.example.com"
                       onChange={e => {
                           setUrlText(e.target.value)
                           onChange({...target, openUrls: splitList(e.target.value)})
                       }}/>
            </Field>
            <Field label="Perfil do Chrome" error={target.chromeProfile && !NAME_PATTERN.test(target.chromeProfile) ? "Use apenas letras, números, '.', '_' ou '-'" : ''}
                   hint="Vazio usa o nome do target. Targets com o mesmo valor compartilham login e cookies, mas não devem ficar abertos ao mesmo tempo.">
                <input value={target.chromeProfile} onChange={set('chromeProfile')} placeholder={target.name || 'qa'}/>
            </Field>
        </div>
    )
}
