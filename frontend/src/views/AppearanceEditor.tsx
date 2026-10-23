import {useState} from 'react'
import {Appearance, Palette} from '../api'
import {Field, MasterDetail} from '../components/Form'
import {Icon} from '../components/Icon'
import {isValidColor} from '../theme'

// Color keys in ssm-helper.json, grouped as they are used in the UI
const colorGroups: {title: string, colors: {key: string, label: string}[]}[] = [
    {
        title: 'Fundos e bordas',
        colors: [
            {key: 'bgDeep', label: 'Fundo da janela (define se a paleta é clara ou escura)'},
            {key: 'bgBase', label: 'Barra lateral, log e campos'},
            {key: 'bgCard', label: 'Cards'},
            {key: 'bgElevated', label: 'Destaques e hover'},
            {key: 'border', label: 'Bordas'},
            {key: 'borderStrong', label: 'Bordas de botões e campos'},
        ],
    },
    {
        title: 'Texto',
        colors: [
            {key: 'textPrimary', label: 'Texto principal'},
            {key: 'textSecondary', label: 'Texto secundário'},
            {key: 'textMuted', label: 'Legendas e rótulos'},
            {key: 'textFaint', label: 'Placeholders e horários'},
        ],
    },
    {
        title: 'Cor principal',
        colors: [
            {key: 'primary', label: 'Botões, abas e itens ativos'},
            {key: 'primary600', label: 'Hover da cor principal'},
            {key: 'primarySoft', label: 'Fundo de itens ativos'},
            {key: 'primaryDeep', label: 'Texto sobre a cor principal, quando ela é clara'},
            {key: 'primaryGlow', label: 'Brilho de destaque'},
        ],
    },
    {
        title: 'Status',
        colors: [
            {key: 'success', label: 'Conectado'},
            {key: 'warning', label: 'Avisos'},
            {key: 'danger', label: 'Erros e desconectar'},
            {key: 'info', label: 'Informações'},
        ],
    },
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
            name: uniqueName(`${base.name.replace(/^ssm-/, '')}-copia`, all.map(p => p.name)),
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
                        title={active ? 'Paleta ativa' : 'Ativar esta paleta'}
                        aria-pressed={active}
                        disabled={!name}
                        onClick={() => !active && onChange({...appearance, palette: name})}
                    >
                        <Icon name="check" size={14}/>
                    </button>
                )
            }}
            addLabel="Nova paleta"
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
                if (palette.builtIn) return 'Paleta padrão do app: não pode ser editada nem excluída.'
                return palette.name === appearance.palette ? `É a paleta ativa; ao remover, o app volta para ${builtins[0]?.name}.` : ''
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
    const readOnly = palette.builtIn
    const setColor = (key: string, value: string) => onChange({...palette, colors: {...palette.colors, [key]: value}})
    const reserved = builtins.some(b => b.name === palette.name)
    const nameError = readOnly ? ''
        : !/^[A-Za-z0-9._-]+$/.test(palette.name) ? "Use apenas letras, números, '.', '_' ou '-'"
            : reserved ? 'Nome reservado para uma paleta padrão' : ''

    return (
        <div className="form-grid">
            <Field label="Nome" error={nameError}
                   hint={readOnly ? <span className="lock-badge"><Icon name="lock" size={11}/> Paleta padrão</span> : undefined}>
                <input value={palette.name} disabled={readOnly} onChange={e => onChange({...palette, name: e.target.value})}/>
            </Field>
            <div className="field-actions">
                <button className="btn" onClick={onDuplicate}><Icon name="copy"/> Duplicar e editar</button>
            </div>

            {colorGroups.map(group => (
                <div key={group.title} className="wide">
                    <h3 className="form-section">{group.title}</h3>
                    <div className="color-grid" style={{marginTop: 10}}>
                        {group.colors.map(({key, label}) => {
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
                                        {valid ? key : `${key}: use #rgb, #rrggbb ou #rrggbbaa`}
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
