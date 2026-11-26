import {useCallback, useRef, useState} from 'react'
import {Icon, IconName} from './Icon'
import {useDismiss} from './useDismiss'

export type Action = {label: string, icon: IconName, danger?: boolean, onSelect: () => void}

// A "..." button with a dropdown of actions, opened toward the left.
export default function ActionsMenu({label, actions, disabled}: {label: string, actions: Action[], disabled?: boolean}) {
    const [open, setOpen] = useState(false)
    const ref = useRef<HTMLDivElement>(null)
    const close = useCallback(() => setOpen(false), [])
    useDismiss(ref, open, close)

    return (
        <div className="actions-menu" ref={ref}>
            <button className={`btn ghost icon-only ${open ? 'open' : ''}`} onClick={() => setOpen(o => !o)}
                    disabled={disabled} aria-haspopup="menu" aria-expanded={open} title={label} aria-label={label}>
                <Icon name="more"/>
            </button>
            {open && (
                <div className="actions-options" role="menu">
                    {actions.map(a => (
                        <button key={a.label} role="menuitem" className={`switcher-option ${a.danger ? 'danger' : ''}`}
                                onClick={() => {
                                    setOpen(false)
                                    a.onSelect()
                                }}>
                            <Icon name={a.icon}/>
                            <span className="switcher-name">{a.label}</span>
                        </button>
                    ))}
                </div>
            )}
        </div>
    )
}
