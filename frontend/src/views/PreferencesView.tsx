import {useEffect, useMemo, useState} from 'react'
import {api, Appearance, localMessage, Message, Palette, toMessage} from '../api'
import {Icon} from '../components/Icon'
import {Field} from '../components/Form'
import {Lang, LANGUAGES, useI18n} from '../i18n'
import AppearanceEditor from './AppearanceEditor'

// Preferences are global: they are stored in ssm-helper.json, not in the environments.
export default function PreferencesView({saved, builtins, onPreview, onSaved}: {
    saved: Appearance
    builtins: Palette[]
    onPreview: (appearance: Appearance | null) => void
    onSaved: (appearance: Appearance) => void
}) {
    const {t, tm, lang, setLang} = useI18n()
    const [draft, setDraft] = useState<Appearance>(saved)
    const [problems, setProblems] = useState<Message[]>([])
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState<Message | null>(null)
    const [savedNotice, setSavedNotice] = useState(false)

    useEffect(() => setDraft(saved), [saved])

    // Palette edits are previewed live; leaving the screen shows the saved palette again
    useEffect(() => onPreview(draft), [draft, onPreview])
    useEffect(() => () => onPreview(null), [onPreview])

    const dirty = useMemo(() => JSON.stringify(draft) !== JSON.stringify(saved), [draft, saved])

    const save = async () => {
        const names = draft.palettes.map(p => p.name.trim())
        const duplicated = names.find((n, i) => names.indexOf(n) !== i)
        if (duplicated !== undefined) {
            setProblems([duplicated
                ? localMessage('LOCAL_PALETTE_DUPLICATE', {name: duplicated})
                : localMessage('LOCAL_PALETTE_UNNAMED')])
            return
        }
        setSaving(true)
        setError(null)
        try {
            const result = await api.saveAppearance(draft)
            setProblems(result)
            if (result.length === 0) {
                setSavedNotice(true)
                onSaved(draft)
            }
        } catch (err) {
            setError(toMessage(err))
        } finally {
            setSaving(false)
        }
    }

    return (
        <div className="page">
            <header className="page-header">
                <div className="spacer">
                    <h1>{t('preferences.title')}</h1>
                    <p className="muted">{t('preferences.subtitle')}</p>
                </div>
            </header>

            {/* Language: applied and saved right away */}
            <div className="card form-card">
                <Field label={t('preferences.language')} hint={t('preferences.languageHint')}>
                    <select value={lang} onChange={e => setLang(e.target.value as Lang)}>
                        {LANGUAGES.map(l => <option key={l.id} value={l.id}>{l.name}</option>)}
                    </select>
                </Field>
            </div>

            <header className="page-header">
                <h2 className="spacer">{t('preferences.appearance')}</h2>
                <button className="btn" onClick={() => {
                    setDraft(saved)
                    setProblems([])
                }} disabled={!dirty || saving}>{t('common.discard')}</button>
                <button className="btn primary" onClick={save} disabled={!dirty || saving}>
                    <Icon name="save"/> {saving ? t('common.saving') : t('common.save')}
                </button>
            </header>

            {problems.length > 0 && (
                <div className="alert warn">
                    <Icon name="alert" size={18}/>
                    <div>
                        <strong>{t('common.fixBeforeSave')}</strong>
                        <ul>{problems.map((p, i) => <li key={i}><code>{tm(p)}</code></li>)}</ul>
                    </div>
                </div>
            )}
            {error && <div className="alert error">{tm(error)}</div>}
            {savedNotice && !dirty && <div className="alert success"><Icon name="check"/> {t('preferences.saved')}</div>}

            <AppearanceEditor appearance={draft} builtins={builtins} onChange={a => {
                setSavedNotice(false)
                setDraft(a)
            }}/>
        </div>
    )
}
