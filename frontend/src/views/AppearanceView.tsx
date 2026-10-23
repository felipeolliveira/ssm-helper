import {useEffect, useMemo, useState} from 'react'
import {api, Appearance, errorMessage, Palette} from '../api'
import {Icon} from '../components/Icon'
import AppearanceEditor from './AppearanceEditor'

// The appearance is global: it is stored in ssm-helper.json, not in the environments.
export default function AppearanceView({saved, builtins, onPreview, onSaved}: {
    saved: Appearance
    builtins: Palette[]
    onPreview: (appearance: Appearance | null) => void
    onSaved: (appearance: Appearance) => void
}) {
    const [draft, setDraft] = useState<Appearance>(saved)
    const [problems, setProblems] = useState<string[]>([])
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState('')
    const [savedNotice, setSavedNotice] = useState(false)

    useEffect(() => setDraft(saved), [saved])

    // Edits are previewed live; leaving the screen shows the saved palette again
    useEffect(() => onPreview(draft), [draft, onPreview])
    useEffect(() => () => onPreview(null), [onPreview])

    const dirty = useMemo(() => JSON.stringify(draft) !== JSON.stringify(saved), [draft, saved])

    const save = async () => {
        const names = draft.palettes.map(p => p.name.trim())
        const duplicated = names.find((n, i) => names.indexOf(n) !== i)
        if (duplicated !== undefined) {
            setProblems([duplicated ? `Paleta '${duplicated}' está duplicada` : 'Há uma paleta sem nome'])
            return
        }
        setSaving(true)
        setError('')
        try {
            const result = await api.saveAppearance(draft)
            setProblems(result)
            if (result.length === 0) {
                setSavedNotice(true)
                onSaved(draft)
            }
        } catch (err) {
            setError(errorMessage(err))
        } finally {
            setSaving(false)
        }
    }

    return (
        <div className="page">
            <header className="page-header">
                <div className="spacer">
                    <h1>Aparência</h1>
                    <p className="muted">Vale para todos os ambientes. Fica salva no <code>ssm-helper.json</code>, ao lado do programa.</p>
                </div>
                <button className="btn" onClick={() => {
                    setDraft(saved)
                    setProblems([])
                }} disabled={!dirty || saving}>Descartar</button>
                <button className="btn primary" onClick={save} disabled={!dirty || saving}>
                    <Icon name="save"/> {saving ? 'Salvando...' : 'Salvar'}
                </button>
            </header>

            {problems.length > 0 && (
                <div className="alert warn">
                    <Icon name="alert" size={18}/>
                    <div>
                        <strong>Corrija antes de salvar:</strong>
                        <ul>{problems.map(p => <li key={p}><code>{p}</code></li>)}</ul>
                    </div>
                </div>
            )}
            {error && <div className="alert error">{error}</div>}
            {savedNotice && !dirty && <div className="alert success"><Icon name="check"/> Aparência salva.</div>}

            <AppearanceEditor appearance={draft} builtins={builtins} onChange={a => {
                setSavedNotice(false)
                setDraft(a)
            }}/>
        </div>
    )
}
