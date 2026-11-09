import {useEffect, useState} from 'react'
import {AboutInfo, api, Message, toMessage} from '../api'
import {Icon} from '../components/Icon'
import {useI18n} from '../i18n'

export default function AboutView() {
    const {t, tm, lang} = useI18n()
    const [info, setInfo] = useState<AboutInfo | null>(null)
    const [error, setError] = useState<Message | null>(null)
    const [copied, setCopied] = useState(false)

    useEffect(() => {
        api.about().then(setInfo).catch(err => setError(toMessage(err)))
    }, [])

    if (error) return <div className="alert error">{tm(error)}</div>
    if (!info) return <div className="empty">{t('about.loading')}</div>

    const commit = info.commit ? info.commit.slice(0, 12) + (info.modified ? ` ${t('about.modified')}` : '') : t('about.unknown')
    const commitDate = info.commitTime
        ? new Date(info.commitTime).toLocaleString(lang, {dateStyle: 'medium', timeStyle: 'short'})
        : t('about.unknown')

    const build: [string, string][] = [
        [t('about.version'), info.version || t('about.unknown')],
        [t('about.buildType'), info.buildType || t('about.unknown')],
        [t('about.commit'), commit],
        [t('about.commitDate'), commitDate],
        [t('about.platform'), info.platform],
    ]
    const components: [string, string][] = [
        [t('about.plugin'), info.pluginVersion],
        [t('about.wails'), info.wailsVersion || t('about.unknown')],
        [t('about.go'), info.goVersion],
    ]
    const folders: [string, string][] = [
        [t('about.appDir'), info.appDir],
        [t('about.dataDir'), info.dataDir],
        [t('about.logDir'), info.logDir],
    ]

    // Plain text for bug reports
    const copy = async () => {
        const lines = [`${info.name} ${info.version}`, ...[...build.slice(1), ...components].map(([k, v]) => `${k}: ${v}`)]
        await navigator.clipboard.writeText(lines.join('\n'))
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    const section = (title: string, rows: [string, string][]) => (
        <section className="card about-card">
            <h2>{title}</h2>
            <dl className="facts">
                {rows.map(([label, value]) => (
                    <div key={label}>
                        <dt>{label}</dt>
                        <dd><code>{value}</code></dd>
                    </div>
                ))}
            </dl>
        </section>
    )

    return (
        <div className="page">
            <header className="page-header about-hero">
                <span className="brand-mark about-mark"><Icon name="tunnel" size={28}/></span>
                <div>
                    <h1>{info.name} <span className="about-version">{info.version}</span></h1>
                    <p className="muted">{info.description}</p>
                    {info.copyright && <p className="muted small">{info.copyright}</p>}
                </div>
                <div className="spacer"/>
                <button className="btn" onClick={copy}>
                    <Icon name={copied ? 'check' : 'copy'}/> {copied ? t('about.copied') : t('about.copy')}
                </button>
                {info.repository && (
                    <button className="btn" onClick={() => api.openURL(`${info.repository}/releases`)}>
                        <Icon name="globe"/> {t('about.releases')}
                    </button>
                )}
            </header>

            {section(t('about.build'), build)}
            {section(t('about.components'), components)}

            <section className="card about-card">
                <header className="page-header">
                    <h2 className="spacer">{t('about.folders')}</h2>
                    <button className="btn" onClick={() => api.openAppFolder().catch(err => setError(toMessage(err)))}>
                        <Icon name="folder"/> {t('about.openAppFolder')}
                    </button>
                </header>
                <dl className="facts">
                    {folders.map(([label, value]) => (
                        <div key={label} className="wide">
                            <dt>{label}</dt>
                            <dd><code>{value}</code></dd>
                        </div>
                    ))}
                </dl>
            </section>
        </div>
    )
}
