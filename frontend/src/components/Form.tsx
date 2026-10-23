import {ReactNode} from 'react'
import {Icon} from './Icon'
import {useI18n} from '../i18n'

export function MasterDetail({items, selected, onSelect, addLabel, onAdd, onRemove, removeWarning, canRemove, itemAction, render}: {
    items: {name: string, sub: ReactNode}[]
    selected: number
    onSelect: (index: number) => void
    addLabel: string
    onAdd: () => void
    onRemove: (index: number) => void
    removeWarning?: (index: number) => string
    canRemove?: (index: number) => boolean
    // Extra control shown at the right of each item in the list
    itemAction?: (index: number) => ReactNode
    render: (index: number) => ReactNode
}) {
    const {t} = useI18n()
    const current = Math.min(selected, items.length - 1)
    const warning = current >= 0 ? removeWarning?.(current) : ''
    const removable = current >= 0 && (canRemove?.(current) ?? true)
    return (
        <div className="master-detail">
            <div className="card master">
                <ul>
                    {items.map((item, i) => (
                        <li key={i} className={i === current ? 'active' : ''}>
                            <button className="master-item" onClick={() => onSelect(i)}>
                                <span className={item.name ? '' : 'muted'}>{item.name || t('common.unnamed')}</span>
                                <span className="muted small">{item.sub}</span>
                            </button>
                            {itemAction?.(i)}
                        </li>
                    ))}
                </ul>
                <button className="btn ghost add" onClick={onAdd}><Icon name="plus"/> {addLabel}</button>
            </div>
            <div className="card detail">
                {current >= 0 ? (
                    <>
                        {render(current)}
                        {(removable || warning) && (
                            <footer className="detail-footer">
                                {warning && <span className="muted small">{warning}</span>}
                                <div className="spacer"/>
                                {removable && (
                                    <button className="btn danger ghost" onClick={() => onRemove(current)}>
                                        <Icon name="trash"/> {t('common.remove')}
                                    </button>
                                )}
                            </footer>
                        )}
                    </>
                ) : (
                    <div className="empty">{t('common.emptyList', {action: addLabel})}</div>
                )}
            </div>
        </div>
    )
}

export function Field({label, hint, error, children, wide}: {
    label: string
    hint?: ReactNode
    error?: string
    children: ReactNode
    wide?: boolean
}) {
    return (
        <label className={`field ${wide ? 'wide' : ''}`}>
            <span className="field-label">{label}</span>
            {children}
            {error ? <span className="field-error">{error}</span> : hint && <span className="field-hint">{hint}</span>}
        </label>
    )
}
