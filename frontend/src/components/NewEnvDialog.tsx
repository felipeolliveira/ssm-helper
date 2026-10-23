import {FormEvent, useState} from 'react'
import {api, EnvList, EnvState, envLabel, Message, toMessage} from '../api'
import {Field} from './Form'
import {Icon} from './Icon'
import {useI18n} from '../i18n'

const NAME_PATTERN = /^[A-Za-z0-9_-]+$/

export default function NewEnvDialog({list, current, onCreated, onClose}: {
    list: EnvList
    current: EnvState | null
    onCreated: (state: EnvState) => void
    onClose: () => void
}) {
    const {t, tm} = useI18n()
    const nameError = (value: string) => (!value || NAME_PATTERN.test(value) ? '' : t('common.nameRuleNoDot'))
    // env.json is only used while there is no env.<name>.json, so it needs a name too
    const needsRename = list.envs.length === 1 && list.envs[0].name === ''
    const canCopy = !!current?.env
    const [name, setName] = useState('')
    const [renameTo, setRenameTo] = useState('')
    const [copyCurrent, setCopyCurrent] = useState(false)
    const [error, setError] = useState<Message | null>(null)
    const [saving, setSaving] = useState(false)

    const taken = list.envs.map(c => c.name)
    const nameProblem = nameError(name) || (name && taken.includes(name) ? t('newEnv.nameTaken') : '')
    const renameProblem = nameError(renameTo) || (renameTo && renameTo === name ? t('newEnv.renameSameName') : '')
    const valid = !!name && !nameProblem && (!needsRename || (!!renameTo && !renameProblem))

    const submit = async (e: FormEvent) => {
        e.preventDefault()
        if (!valid) return
        setSaving(true)
        setError(null)
        try {
            onCreated(await api.createEnv({name, copyCurrent, renameCurrentTo: needsRename ? renameTo : ''}))
        } catch (err) {
            setError(toMessage(err))
            setSaving(false)
        }
    }

    return (
        <div className="dialog-backdrop" onMouseDown={e => e.target === e.currentTarget && onClose()}>
            <form className="dialog card" onSubmit={submit}>
                <header className="dialog-header">
                    <h2>{t('newEnv.title')}</h2>
                    <button type="button" className="btn ghost icon-only" onClick={onClose} title={t('common.close')}><Icon name="x"/></button>
                </header>

                <Field label={t('newEnv.name')} error={nameProblem}
                       hint={t('newEnv.nameHint', {name: name || t('newEnv.namePlaceholder')})}>
                    <input autoFocus value={name} onChange={e => setName(e.target.value.trim())} placeholder="qa"/>
                </Field>

                {needsRename && (
                    <Field label={t('newEnv.renameCurrent')} error={renameProblem}
                           hint={t('newEnv.renameCurrentHint', {name: renameTo || t('newEnv.namePlaceholder')})}>
                        <input value={renameTo} onChange={e => setRenameTo(e.target.value.trim())} placeholder="padrao"/>
                    </Field>
                )}

                <div className="field">
                    <span className="field-label">{t('newEnv.startWith')}</span>
                    <label className="radio">
                        <input type="radio" checked={!copyCurrent} onChange={() => setCopyCurrent(false)}/>
                        {t('newEnv.empty')}
                    </label>
                    <label className={`radio ${canCopy ? '' : 'muted'}`}>
                        <input type="radio" checked={copyCurrent} disabled={!canCopy} onChange={() => setCopyCurrent(true)}/>
                        {t('newEnv.copyOf', {env: current ? envLabel(current.name) : ''})}
                    </label>
                </div>

                {error && <div className="alert error">{tm(error)}</div>}

                <footer className="dialog-footer">
                    <button type="button" className="btn ghost" onClick={onClose}>{t('common.cancel')}</button>
                    <button type="submit" className="btn primary" disabled={!valid || saving}>
                        <Icon name="plus"/> {saving ? t('newEnv.creating') : t('newEnv.create')}
                    </button>
                </footer>
            </form>
        </div>
    )
}
