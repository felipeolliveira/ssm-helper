import {createContext, ReactNode, useCallback, useContext, useEffect, useState} from 'react'
import {api} from './api'

// Hides account numbers, hosts, domains and other environment details on
// screen, e.g. while sharing it. The choice is stored in ssm-helper.json.
type Privacy = {hidden: boolean, toggle: () => void}

const PrivacyContext = createContext<Privacy>({hidden: false, toggle: () => {}})

export function PrivacyProvider({children}: {children: ReactNode}) {
    // Hidden until the stored choice loads, so nothing shows by accident
    const [hidden, setHidden] = useState(true)

    useEffect(() => {
        api.getPreferences().then(p => setHidden(p.hideSensitive)).catch(() => setHidden(false))
    }, [])

    const toggle = useCallback(() => setHidden(h => {
        api.setHideSensitive(!h).catch(() => {})
        return !h
    }), [])
    return <PrivacyContext.Provider value={{hidden, toggle}}>{children}</PrivacyContext.Provider>
}

export function usePrivacy() {
    return useContext(PrivacyContext)
}

// Masked values always have the same length, so neither the size nor the shape shows.
export const MASK = '••••••••'

// keepLast shows the end of the value, like a bank account (••••••••6278).
export function mask(value: string, keepLast = 0) {
    return keepLast > 0 && value.length > keepLast ? MASK + value.slice(-keepLast) : MASK
}

// Renders a sensitive value, masked while privacy mode is on.
export function Secret({value, keepLast}: {value: string, keepLast?: number}) {
    const {hidden} = usePrivacy()
    return <>{hidden && value ? mask(value, keepLast) : value}</>
}

// Props for an input with a sensitive value: while privacy mode is on it shows
// MASK and is read-only, since editing a value you cannot see is error-prone.
export function useSecretInput() {
    const {hidden} = usePrivacy()
    return (value: string) => hidden
        ? {value: value ? MASK : '', readOnly: true, className: 'secret-input', title: 'Mostre os dados sensíveis (ícone de olho) para editar'}
        : {}
}

// Class for free text with sensitive content (log lines, error messages).
export function useSecretText() {
    return usePrivacy().hidden ? 'secret-text' : ''
}
