import {FormEvent, useState} from 'react'
import {api, EnvList, EnvState, envLabel, errorMessage} from '../api'
import {Field} from './Form'
import {Icon} from './Icon'

const NAME_PATTERN = /^[A-Za-z0-9_-]+$/

function nameError(name: string) {
    if (!name) return ''
    return NAME_PATTERN.test(name) ? '' : "Use apenas letras, números, '_' ou '-'"
}

export default function NewEnvDialog({list, current, onCreated, onClose}: {
    list: EnvList
    current: EnvState | null
    onCreated: (state: EnvState) => void
    onClose: () => void
}) {
    // env.json is only used while there is no env.<name>.json, so it needs a name too
    const needsRename = list.envs.length === 1 && list.envs[0].name === ''
    const canCopy = !!current?.env
    const [name, setName] = useState('')
    const [renameTo, setRenameTo] = useState('')
    const [copyCurrent, setCopyCurrent] = useState(false)
    const [error, setError] = useState('')
    const [saving, setSaving] = useState(false)

    const taken = list.envs.map(c => c.name)
    const nameProblem = nameError(name) || (name && taken.includes(name) ? 'Já existe um ambiente com esse nome' : '')
    const renameProblem = nameError(renameTo) || (renameTo && renameTo === name ? 'Use um nome diferente da nova' : '')
    const valid = !!name && !nameProblem && (!needsRename || (!!renameTo && !renameProblem))

    const submit = async (e: FormEvent) => {
        e.preventDefault()
        if (!valid) return
        setSaving(true)
        setError('')
        try {
            onCreated(await api.createEnv({name, copyCurrent, renameCurrentTo: needsRename ? renameTo : ''}))
        } catch (err) {
            setError(errorMessage(err))
            setSaving(false)
        }
    }

    return (
        <div className="dialog-backdrop" onMouseDown={e => e.target === e.currentTarget && onClose()}>
            <form className="dialog card" onSubmit={submit}>
                <header className="dialog-header">
                    <h2>Novo ambiente</h2>
                    <button type="button" className="btn ghost icon-only" onClick={onClose} title="Fechar"><Icon name="x"/></button>
                </header>

                <Field label="Nome" error={nameProblem}
                       hint={<>Será salvo como <code>env.{name || '<nome>'}.json</code> na pasta do programa.</>}>
                    <input autoFocus value={name} onChange={e => setName(e.target.value.trim())} placeholder="qa"/>
                </Field>

                {needsRename && (
                    <Field label="Nome para o ambiente atual" error={renameProblem}
                           hint={<>O <code>env.json</code> só é usado enquanto não há outros ambientes, então ele será renomeado para <code>env.{renameTo || '<nome>'}.json</code>.</>}>
                        <input value={renameTo} onChange={e => setRenameTo(e.target.value.trim())} placeholder="padrao"/>
                    </Field>
                )}

                <div className="field">
                    <span className="field-label">Começar com</span>
                    <label className="radio">
                        <input type="radio" checked={!copyCurrent} onChange={() => setCopyCurrent(false)}/>
                        Ambiente vazio
                    </label>
                    <label className={`radio ${canCopy ? '' : 'muted'}`}>
                        <input type="radio" checked={copyCurrent} disabled={!canCopy} onChange={() => setCopyCurrent(true)}/>
                        Cópia de {current ? envLabel(current.name) : 'atual'} (perfis e targets)
                    </label>
                </div>

                {error && <div className="alert error">{error}</div>}

                <footer className="dialog-footer">
                    <button type="button" className="btn ghost" onClick={onClose}>Cancelar</button>
                    <button type="submit" className="btn primary" disabled={!valid || saving}>
                        <Icon name="plus"/> {saving ? 'Criando...' : 'Criar'}
                    </button>
                </footer>
            </form>
        </div>
    )
}
