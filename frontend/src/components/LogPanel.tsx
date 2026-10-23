import {useEffect, useRef} from 'react'
import {LogEntry} from '../api'
import {Icon} from './Icon'
import {useSecretText} from '../privacy'

export default function LogPanel({open, logs, onClose, onClear}: {
    open: boolean
    logs: LogEntry[]
    onClose: () => void
    onClear: () => void
}) {
    const endRef = useRef<HTMLDivElement>(null)
    // Log lines carry session IDs, hosts and the Chrome command line
    const secretText = useSecretText()

    useEffect(() => {
        if (open) endRef.current?.scrollIntoView({block: 'end'})
    }, [open, logs.length])

    if (!open) return null
    return (
        <section className="log-panel">
            <header>
                <strong>Log</strong>
                <span className="muted">Mensagens do login SSO e dos túneis (em inglês quando vêm da AWS)</span>
                <div className="spacer"/>
                <button className="btn ghost" onClick={onClear} disabled={logs.length === 0}>Limpar</button>
                <button className="btn ghost icon-only" onClick={onClose} title="Fechar"><Icon name="x"/></button>
            </header>
            <div className="log-lines">
                {logs.length === 0 && <div className="muted">Nenhuma mensagem ainda.</div>}
                {logs.map((entry, i) => (
                    <div key={i} className="log-line">
                        <span className="log-time">{entry.time}</span>
                        <span className="log-source">{entry.source}</span>
                        <span className={`log-text ${secretText}`}>{entry.text}</span>
                    </div>
                ))}
                <div ref={endRef}/>
            </div>
        </section>
    )
}
