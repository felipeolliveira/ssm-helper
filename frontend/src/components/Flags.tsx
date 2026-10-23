import {ReactElement} from 'react'

// Flags by language code. They are drawn as SVG because Windows shows flag
// emojis as plain letters. To add a language, add its flag here.
const definedFlags: Record<string, ReactElement> = {
    'pt-BR': (
        <svg className="flag" viewBox="0 0 20 14" aria-hidden="true">
            <rect width="20" height="14" fill="#009b3a"/>
            <polygon points="10,1.6 18.4,7 10,12.4 1.6,7" fill="#fedf00"/>
            <circle cx="10" cy="7" r="3.1" fill="#002776"/>
            <path d="M7.1 6.2a4.4 4.4 0 0 1 5.8 1.4" stroke="#fff" strokeWidth="0.6" fill="none"/>
        </svg>
    ),
    'en-US': (
        <svg className="flag" viewBox="0 0 20 14" aria-hidden="true">
            <rect width="20" height="14" fill="#fff"/>
            {[0, 2, 4, 6, 8, 10, 12].map(i => <rect key={i} y={i * 14 / 13} width="20" height={14 / 13} fill="#b22234"/>)}
            <rect width="8.4" height={14 * 7 / 13} fill="#3c3b6e"/>
            {[1.2, 3.2, 5.2, 7.2].flatMap(x => [1.1, 2.9, 4.7, 6.5].map(y => (
                <circle key={`${x}-${y}`} cx={x} cy={y} r="0.38" fill="#fff"/>
            )))}
        </svg>
    ),
}

const FALLBACK = 'en-US'

// flags[lang] is a direct lookup; a language without its own flag (e.g. 'en'
// or 'es') gets the en-US one.
export const flags: Record<string, ReactElement> = new Proxy(definedFlags, {
    get: (target, key) => (typeof key === 'string' && key in target ? target[key] : target[FALLBACK]),
})
