import {useState} from 'react'
import {Appearance, Palette} from '../api'
import {Field, MasterDetail} from '../components/Form'
import {Icon} from '../components/Icon'
import {isValidColor} from '../theme'
import {useI18n} from '../i18n'

// Color keys in ssm-helper.json, grouped as they are used in the UI; labels
// come from the "colors" section of the dictionaries.
type ColorKey = 'bgDeep' | 'bgBase' | 'bgCard' | 'bgElevated' | 'border' | 'borderStrong'
    | 'textPrimary' | 'textSecondary' | 'textMuted' | 'textFaint'
    | 'primary' | 'primary600' | 'primarySoft' | 'primaryDeep' | 'primaryGlow'
    | 'success' | 'warning' | 'danger' | 'info'

const colorGroups: {group: 'backgrounds' | 'text' | 'primary' | 'status', keys: ColorKey[]}[] = [
    {group: 'backgrounds', keys: ['bgDeep', 'bgBase', 'bgCard', 'bgElevated', 'border', 'borderStrong']},
    {group: 'text', keys: ['textPrimary', 'textSecondary', 'textMuted', 'textFaint']},
    {group: 'primary', keys: ['primary', 'primary600', 'primarySoft', 'primaryDeep', 'primaryGlow']},
    {group: 'status', keys: ['success', 'warning', 'danger', 'info']},
]

const previewKeys = ['bgDeep', 'bgCard', 'primary', 'textPrimary', 'success', 'danger']

// The native color picker only takes #rrggbb; the alpha suffix is kept apart.
function splitHex(value: string): {rgb: string, alpha: string} {
    if (!isValidColor(value)) return {rgb: '#000000', alpha: ''}
    let hex = value.slice(1)
    if (hex.length === 3) hex = hex.split('').map(c => c + c).join('')
    return {rgb: `#${hex.slice(0, 6)}`, alpha: hex.slice(6)}
}

function uniqueName(base: string, taken: string[]) {
    let name = base
    for (let i = 2; taken.includes(name); i++) name = `${base}-${i}`
    return name
}

export default function AppearanceEditor({appearance, builtins, onChange}: {
    appearance: Appearance
    builtins: Palette[]
    onChange: (appearance: Appearance) => void
}) {
    const {t} = useI18n()
    // Built-in palettes come first and are read-only; the others are stored in ssm-helper.json
    const all = [...builtins, ...appearance.palettes]
    const [selected, setSelected] = useState(() => Math.max(0, all.findIndex(p => p.name === appearance.palette)))
    const customIndex = (index: number) => index - builtins.length

    const setCustom = (index: number, palette: Palette) => {
        const previous = all[index]
        onChange({
            // Renaming the active palette keeps it active
            palette: appearance.palette === previous.name ? palette.name : appearance.palette,
            palettes: appearance.palettes.map((p, i) => (i === customIndex(index) ? palette : p)),
        })
    }

    // New palettes start as a copy of an existing one
    const duplicate = (index: number) => {
        const base = all[index] ?? builtins[0]
        if (!base) return
        const copy: Palette = {
            name: uniqueName(`${base.name.replace(/^ssm-/, '')}-${t('palette.copySuffix')}`, all.map(p => p.name)),
            colors: {...base.colors},
            builtIn: false,
        }
        onChange({...appearance, palettes: [...appearance.palettes, copy]})
        setSelected(all.length)
    }

    return (
        <MasterDetail
            items={all.map(p => ({
                name: p.name,
                sub: (
                    <span className="palette-preview">
                        {previewKeys.map(k => <span key={k} style={{background: p.colors[k]}}/>)}
                    </span>
                ),
            }))}
            selected={selected}
            onSelect={setSelected}
            itemAction={index => {
                const name = all[index].name
                const active = name === appearance.palette
                return (
                    <button
                        className={`activate-btn ${active ? 'active' : ''}`}
                        title={active ? t('palette.active') : t('palette.activate')}
                        aria-pressed={active}
                        disabled={!name}
                        onClick={() => !active && onChange({...appearance, palette: name})}
                    >
                        <Icon name="check" size={14}/>
                    </button>
                )
            }}
            addLabel={t('palette.new')}
            onAdd={() => duplicate(selected)}
            canRemove={index => !all[index].builtIn}
            onRemove={index => {
                const removed = all[index]
                onChange({
                    palette: removed.name === appearance.palette ? (builtins[0]?.name ?? appearance.palette) : appearance.palette,
                    palettes: appearance.palettes.filter((_, i) => i !== customIndex(index)),
                })
                setSelected(Math.max(0, index - 1))
            }}
            removeWarning={index => {
                const palette = all[index]
                if (palette.builtIn) return t('palette.builtInLocked')
                return palette.name === appearance.palette ? t('palette.activeRemoveWarning', {palette: builtins[0]?.name ?? ''}) : ''
            }}
            render={index => (
                <PaletteForm
                    key={index}
                    palette={all[index]}
                    builtins={builtins}
                    onChange={palette => setCustom(index, palette)}
                    onDuplicate={() => duplicate(index)}
                />
            )}
        />
    )
}

function PaletteForm({palette, builtins, onChange, onDuplicate}: {
    palette: Palette
    builtins: Palette[]
    onChange: (p: Palette) => void
    onDuplicate: () => void
}) {
    const {t} = useI18n()
    const readOnly = palette.builtIn
    const setColor = (key: string, value: string) => onChange({...palette, colors: {...palette.colors, [key]: value}})
    const reserved = builtins.some(b => b.name === palette.name)
    const nameError = readOnly ? ''
        : !/^[A-Za-z0-9._-]+$/.test(palette.name) ? t('common.nameRule')
            : reserved ? t('palette.reservedName') : ''

    return (
        <div className="form-grid">
            <Field label={t('palette.name')} error={nameError}
                   hint={readOnly ? <span className="lock-badge"><Icon name="lock" size={11}/> {t('palette.builtIn')}</span> : undefined}>
                <input value={palette.name} disabled={readOnly} onChange={e => onChange({...palette, name: e.target.value})}/>
            </Field>
            <div className="field-actions">
                <button className="btn" onClick={onDuplicate}><Icon name="copy"/> {t('palette.duplicate')}</button>
            </div>

            {colorGroups.map(({group, keys}) => (
                <div key={group} className="wide">
                    <h3 className="form-section">{t(`colors.groups.${group}`)}</h3>
                    <div className="color-grid" style={{marginTop: 10}}>
                        {keys.map(key => {
                            const label = t(`colors.${key}`)
                            const value = palette.colors[key] ?? ''
                            const valid = isValidColor(value)
                            const {rgb, alpha} = splitHex(value)
                            return (
                                <label key={key} className="color-row">
                                    <span className="color-swatch">
                                        <span style={{background: valid ? value : 'transparent'}}/>
                                        <input type="color" value={rgb} disabled={readOnly} aria-label={label}
                                               onChange={e => setColor(key, e.target.value + alpha)}/>
                                    </span>
                                    <span className="field-label">{label}</span>
                                    <input type="text" value={value} spellCheck={false} disabled={readOnly}
                                           onChange={e => setColor(key, e.target.value.trim())}/>
                                    <span className={valid ? 'field-hint' : 'field-error'}>
                                        {valid ? key : t('palette.colorRule', {key})}
                                    </span>
                                </label>
                            )
                        })}
                    </div>
                </div>
            ))}
        </div>
    )
}
