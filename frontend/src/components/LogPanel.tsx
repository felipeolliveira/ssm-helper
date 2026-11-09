import {useEffect, useRef} from 'react'
import {LogEntry} from '../api'
import {Icon} from './Icon'
import {useSecretText} from '../privacy'
import {useI18n} from '../i18n'

export default function LogPanel({open, logs, onClose, onClear, onOpenFolder}: {
    open: boolean
    logs: LogEntry[]
    onClose: () => void
    onClear: () => void
    onOpenFolder: () => void
}) {
    const {t, tm} = useI18n()
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
                <strong>{t('log.title')}</strong>
                <span className="muted">{t('log.subtitle')}</span>
                <div className="spacer"/>
                <button className="btn ghost" onClick={onOpenFolder} title={t('log.openFolderHint')}>
                    <Icon name="folder"/> {t('log.openFolder')}
                </button>
                <button className="btn ghost" onClick={onClear} disabled={logs.length === 0}>{t('log.clear')}</button>
                <button className="btn ghost icon-only" onClick={onClose} title={t('common.close')}><Icon name="x"/></button>
            </header>
            <div className="log-lines">
                {logs.length === 0 && <div className="muted">{t('log.empty')}</div>}
                {logs.map((entry, i) => (
                    <div key={i} className="log-line">
                        <span className="log-time">{entry.time}</span>
                        <span className="log-source">{entry.source}</span>
                        <span className={`log-text ${secretText}`}>{tm(entry.message)}</span>
                    </div>
                ))}
                <div ref={endRef}/>
            </div>
        </section>
    )
}
