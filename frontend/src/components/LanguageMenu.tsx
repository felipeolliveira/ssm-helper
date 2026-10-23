import {useEffect, useRef, useState} from 'react'
import {LANGUAGES, useI18n} from '../i18n'
import {Icon} from './Icon'
import {flags} from './flags'

// Language picker: a button with the current flag and a dropdown with the others.
export default function LanguageMenu() {
    const {lang, setLang, t} = useI18n()
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

    const current = LANGUAGES.find(l => l.id === lang) ?? LANGUAGES[0]

    return (
        <div className="language-menu" ref={ref}>
            <button className={`btn language-button ${open ? 'open' : ''}`} onClick={() => setOpen(o => !o)}
                    aria-haspopup="listbox" aria-expanded={open} title={t('preferences.language')}>
                {flags[current.id]}
                <span>{current.name}</span>
                <Icon name="chevron"/>
            </button>
            {open && (
                <div className="language-options" role="listbox" aria-label={t('preferences.language')}>
                    {LANGUAGES.map(l => (
                        <button key={l.id} role="option" aria-selected={l.id === lang}
                                className={`switcher-option ${l.id === lang ? 'active' : ''}`}
                                onClick={() => {
                                    setOpen(false)
                                    if (l.id !== lang) setLang(l.id)
                                }}>
                            {flags[l.id]}
                            <span className="switcher-name">{l.name}</span>
                            <span className="switcher-check">{l.id === lang && <Icon name="check" size={14}/>}</span>
                        </button>
                    ))}
                </div>
            )}
        </div>
    )
}
