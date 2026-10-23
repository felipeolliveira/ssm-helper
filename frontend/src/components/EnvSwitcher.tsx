import {useEffect, useRef, useState} from 'react'
import {EnvList, envLabel} from '../api'
import {Icon} from './Icon'

// Environment selector in the sidebar; tunnels of other environments keep running.
export default function EnvSwitcher({list, onSelect, onNew}: {
    list: EnvList | null
    onSelect: (name: string) => void
    onNew: () => void
}) {
    const [open, setOpen] = useState(false)
    const ref = useRef<HTMLDivElement>(null)

    useEffect(() => {
        if (!open) return
        const close = (e: MouseEvent) => {
            if (!ref.current?.contains(e.target as Node)) setOpen(false)
        }
        const escape = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
        document.addEventListener('mousedown', close)
        document.addEventListener('keydown', escape)
        return () => {
            document.removeEventListener('mousedown', close)
            document.removeEventListener('keydown', escape)
        }
    }, [open])

    const envs = list?.envs ?? []
    const otherTunnels = envs.filter(c => c.name !== list?.active).reduce((n, c) => n + c.activeTunnels, 0)

    return (
        <div className="env-switcher" ref={ref}>
            <span className="switcher-label">Ambiente ativo</span>
            <button className={`switcher-button ${open ? 'open' : ''}`} onClick={() => setOpen(o => !o)}
                    aria-haspopup="listbox" aria-expanded={open}>
                <Icon name="file"/>
                <span className="truncate">{list ? (envs.length ? envLabel(list.active) : 'Nenhum') : '...'}</span>
                {otherTunnels > 0 && (
                    <span className="nav-badge" title="Túneis abertos em outros ambientes">{otherTunnels}</span>
                )}
                <Icon name="chevron"/>
            </button>
            {open && (
                <div className="switcher-menu" role="listbox">
                    {envs.map(c => (
                        <button key={c.name} role="option" aria-selected={c.name === list?.active}
                                className={`switcher-option ${c.name === list?.active ? 'active' : ''}`}
                                onClick={() => {
                                    setOpen(false)
                                    if (c.name !== list?.active) onSelect(c.name)
                                }}>
                            <span className="switcher-check">{c.name === list?.active && <Icon name="check" size={14}/>}</span>
                            <span className="switcher-name">
                                <span className="truncate">{envLabel(c.name)}</span>
                                <span className="muted small truncate">{c.file}</span>
                            </span>
                            {c.activeTunnels > 0 && (
                                <span className="nav-badge" title="Túneis abertos">{c.activeTunnels}</span>
                            )}
                        </button>
                    ))}
                    {envs.length > 0 && <div className="switcher-divider"/>}
                    <button className="switcher-option" onClick={() => {
                        setOpen(false)
                        onNew()
                    }}>
                        <span className="switcher-check"><Icon name="plus" size={14}/></span>
                        <span className="switcher-name">Novo ambiente</span>
                    </button>
                </div>
            )}
        </div>
    )
}
