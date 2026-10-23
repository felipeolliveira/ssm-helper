// Checks that every message code declared in internal/msg/codes.go has a text
// in each dictionary of frontend/src/i18n/locales. Run by `make test`.
import {readFileSync, readdirSync} from 'node:fs'
import {join} from 'node:path'
import {fileURLToPath} from 'node:url'

// fileURLToPath also works on Windows, where URL.pathname gives /C:/...
const root = fileURLToPath(new URL('..', import.meta.url))
const codesFile = readFileSync(join(root, 'internal/msg/codes.go'), 'utf8')
const codes = [...codesFile.matchAll(/^\s*\w+\s*=\s*"([A-Z0-9_]+)"/gm)].map(m => m[1])

const localesDir = join(root, 'frontend/src/i18n/locales')
let missing = 0
for (const file of readdirSync(localesDir).filter(f => f.endsWith('.json'))) {
    const messages = JSON.parse(readFileSync(join(localesDir, file), 'utf8')).messages ?? {}
    for (const code of codes) {
        if (!(code in messages)) {
            console.error(`${file}: missing message for ${code}`)
            missing++
        }
    }
}
if (missing > 0) process.exit(1)
console.log(`i18n: ${codes.length} message codes translated in every locale`)
