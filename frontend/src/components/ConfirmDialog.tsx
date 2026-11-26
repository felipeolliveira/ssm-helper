import {ReactNode} from 'react'
import {Icon, IconName} from './Icon'
import {useI18n} from '../i18n'

// Asks before an action, destructive by default. Escape and a click outside cancel it.
export default function ConfirmDialog({title, children, confirmLabel, confirmIcon, danger = true, onConfirm, onCancel}: {
    title: string
    children: ReactNode
    confirmLabel: string
    confirmIcon: IconName
    danger?: boolean
    onConfirm: () => void
    onCancel: () => void
}) {
    const {t} = useI18n()
    return (
        <div className="dialog-backdrop" onMouseDown={e => e.target === e.currentTarget && onCancel()}
             onKeyDown={e => e.key === 'Escape' && onCancel()}>
            <div className="dialog card" role="alertdialog" aria-modal="true" aria-label={title}>
                <header className="dialog-header">
                    <h2>{title}</h2>
                    <button type="button" className="btn ghost icon-only" onClick={onCancel} title={t('common.close')}><Icon name="x"/></button>
                </header>
                <div className="dialog-body">{children}</div>
                <footer className="dialog-footer">
                    <button type="button" className="btn ghost" onClick={onCancel}>{t('common.cancel')}</button>
                    <button type="button" className={`btn ${danger ? 'danger' : 'primary'}`} autoFocus onClick={onConfirm}>
                        <Icon name={confirmIcon}/> {confirmLabel}
                    </button>
                </footer>
            </div>
        </div>
    )
}
