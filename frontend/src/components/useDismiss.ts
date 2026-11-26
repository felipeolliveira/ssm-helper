import {RefObject, useEffect} from 'react'

// Closes a popup on a click outside ref or on Escape, while it is open.
export function useDismiss(ref: RefObject<HTMLElement | null>, open: boolean, close: () => void) {
    useEffect(() => {
        if (!open) return
        const outside = (e: MouseEvent) => {
            if (!ref.current?.contains(e.target as Node)) close()
        }
        const escape = (e: KeyboardEvent) => e.key === 'Escape' && close()
        document.addEventListener('mousedown', outside)
        document.addEventListener('keydown', escape)
        return () => {
            document.removeEventListener('mousedown', outside)
            document.removeEventListener('keydown', escape)
        }
    }, [ref, open, close])
}
