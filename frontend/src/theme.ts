import {Appearance, Palette} from './api'
import {WindowSetDarkTheme, WindowSetLightTheme} from '../wailsjs/runtime/runtime'

const HEX_COLOR = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/

export function isValidColor(value: string) {
    return HEX_COLOR.test(value)
}

// Relative luminance (0 dark .. 1 light), ignoring alpha.
function luminance(hex: string): number {
    if (!isValidColor(hex)) return 0
    let digits = hex.slice(1)
    if (digits.length === 3) digits = digits.split('').map(c => c + c).join('')
    const [r, g, b] = [0, 2, 4].map(i => parseInt(digits.slice(i, i + 2), 16) / 255)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

// A palette is light or dark according to its window background.
export function isLightPalette(palette: Palette) {
    return luminance(palette.colors.bgDeep) > 0.5
}

// bgDeep -> --bg-deep, primary600 -> --primary-600
function cssVariable(key: string) {
    return '--' + key.replace(/([a-z])([A-Z0-9])/g, '$1-$2').toLowerCase()
}

export function activePalette(appearance: Appearance, builtins: Palette[]): Palette | undefined {
    return [...builtins, ...appearance.palettes].find(p => p.name === appearance.palette) ?? builtins[0]
}

export function applyPalette(palette: Palette) {
    const root = document.documentElement
    for (const [key, value] of Object.entries(palette.colors)) {
        // Half-typed colors in the editor are skipped until they are valid
        if (isValidColor(value)) root.style.setProperty(cssVariable(key), value)
    }
    const light = isLightPalette(palette)
    root.dataset.theme = light ? 'light' : 'dark'
    root.style.colorScheme = light ? 'light' : 'dark'
    // Text on the primary color: dark on bright primaries, white otherwise
    root.style.setProperty('--on-primary', luminance(palette.colors.primary) > 0.4
        ? palette.colors.primaryDeep
        : '#ffffff')
    try {
        if (light) WindowSetLightTheme()
        else WindowSetDarkTheme()
    } catch {
        // runtime not available outside Wails
    }
}
