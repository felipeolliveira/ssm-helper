import {createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useState} from 'react'
import ptBR from './locales/pt-BR.json'
import en from './locales/en.json'
import {api, Message} from '../api'

// Dictionaries: "ui" holds the screen texts, "messages" a text for each code
// the backend sends (internal/msg/codes.go). Parameters are written as {name}.
type Dictionary = typeof ptBR

// Both dictionaries must have the same keys; a missing one fails `tsc`.
const enChecked: Dictionary = en
const ptChecked: typeof en = ptBR

export type Lang = 'pt-BR' | 'en'

const dictionaries: Record<Lang, Dictionary> = {'pt-BR': ptChecked as Dictionary, en: enChecked}

export const LANGUAGES: {id: Lang, name: string}[] = [
    {id: 'pt-BR', name: ptBR.language.name},
    {id: 'en', name: en.language.name},
]

// Dotted paths of the screen texts, e.g. "targets.connect"
type Leaves<T, P extends string = ''> = {
    [K in keyof T & string]: T[K] extends string ? `${P}${K}` : Leaves<T[K], `${P}${K}.`>
}[keyof T & string]
export type UIKey = Leaves<Dictionary['ui']>

type Params = Record<string, string | number>

// The Windows language decides the first one: Portuguese or English
export function systemLanguage(): Lang {
    return navigator.language.toLowerCase().startsWith('pt') ? 'pt-BR' : 'en'
}

function interpolate(text: string, params?: Params) {
    return text.replace(/\{(\w+)\}/g, (_, name) => (params && name in params ? String(params[name]) : ''))
}

function lookup(dict: Dictionary, key: string): string | undefined {
    let node: unknown = dict.ui
    for (const part of key.split('.')) {
        if (typeof node !== 'object' || node === null) return undefined
        node = (node as Record<string, unknown>)[part]
    }
    return typeof node === 'string' ? node : undefined
}

type I18n = {
    lang: Lang
    setLang: (lang: Lang) => void
    // Screen text
    t: (key: UIKey, params?: Params) => string
    // Message from the backend (or built in the frontend) translated to text
    tm: (message: Message) => string
    formatDateTime: (iso: string) => string
}

const I18nContext = createContext<I18n | null>(null)

export function I18nProvider({children}: {children: ReactNode}) {
    const [lang, setLangState] = useState<Lang>(systemLanguage)

    useEffect(() => {
        api.getPreferences().then(p => {
            if (p.language === 'pt-BR' || p.language === 'en') {
                setLangState(p.language)
            } else {
                // First run: keep the Windows language and remember it
                api.setLanguage(systemLanguage()).catch(() => {})
            }
        }).catch(() => {})
    }, [])

    useEffect(() => {
        document.documentElement.lang = lang
    }, [lang])

    const setLang = useCallback((next: Lang) => {
        setLangState(next)
        api.setLanguage(next).catch(() => {})
    }, [])

    const value = useMemo<I18n>(() => {
        const dict = dictionaries[lang]
        const messages = dict.messages as Record<string, string>
        return {
            lang,
            setLang,
            t: (key, params) => interpolate(lookup(dict, key) ?? key, params),
            tm: message => {
                const template = messages[message.code]
                const params: Params = {...(message.params ?? {}), detail: message.detail ?? ''}
                if (template === undefined) {
                    // Unknown code: show it as is, never a blank line
                    return message.detail ? `${message.code}: ${message.detail}` : message.code
                }
                let text = interpolate(template, params).replace(/[\s:]+$/, '').trim()
                // The technical detail (e.g. the AWS error) is appended when the text does not place it
                if (message.detail && !template.includes('{detail}')) text += `: ${message.detail}`
                return text
            },
            formatDateTime: iso => iso
                ? new Date(iso).toLocaleString(lang, {day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit'})
                : '',
        }
    }, [lang, setLang])

    return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18n {
    const i18n = useContext(I18nContext)
    if (!i18n) throw new Error('useI18n must be used inside I18nProvider')
    return i18n
}
