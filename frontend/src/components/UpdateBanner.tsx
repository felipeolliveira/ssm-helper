import {useEffect, useState} from 'react'
import {api, events, Message, toMessage, UpdateInfo, UpdateProgress} from '../api'
import ConfirmDialog from './ConfirmDialog'
import {Icon} from './Icon'
import {useI18n} from '../i18n'

// Offers a newer release: see what changed, update and restart, or hide it
// until the app opens again.
export default function UpdateBanner({info, activeTunnels, onDismiss}: {
    info: UpdateInfo | null
    activeTunnels: number
    onDismiss: () => void
}) {
    const {t, tm} = useI18n()
    const [confirm, setConfirm] = useState(false)
    const [applying, setApplying] = useState(false)
    const [progress, setProgress] = useState<UpdateProgress | null>(null)
    const [error, setError] = useState<Message | null>(null)

    useEffect(() => events.onUpdateProgress(setProgress), [])

    if (info?.blocked) {
        // A release that may have been tampered with: no install and no download link
        return (
            <div className="sso-banners">
                <div className="alert error update-banner">
                    <Icon name="alert" size={18}/>
                    <div className="sso-text">
                        <strong>{t('update.blockedTitle', {version: info.latest})}</strong>
                        <span>{t('update.blockedText')}</span>
                    </div>
                    <div className="spacer"/>
                    <button className="btn ghost" onClick={onDismiss}>{t('update.understood')}</button>
                </div>
            </div>
        )
    }
    if (!info?.available) return null

    const apply = async () => {
        setConfirm(false)
        setApplying(true)
        setError(null)
        setProgress(null)
        try {
            await api.applyUpdate()
            // The app closes and the new version opens
        } catch (err) {
            setError(toMessage(err))
            setApplying(false)
        }
    }

    const percent = progress && progress.total > 0 ? Math.round(progress.done / progress.total * 100) : null

    return (
        <div className="sso-banners">
            <div className="alert info sso-banner update-banner">
                <Icon name="refresh" size={18}/>
                <div className="sso-text">
                    <strong>{t('update.available', {version: info.latest})}</strong>
                    <span>
                        {applying
                            ? (percent === null ? t('update.downloading') : t('update.downloadingPercent', {percent: String(percent)}))
                            : t('update.current', {version: info.current})}
                    </span>
                    {applying && <progress className="update-progress" max={100} value={percent ?? undefined}/>}
                    {error && <span className="text-error">{tm(error)}</span>}
                </div>
                <div className="spacer"/>
                <button className="btn ghost" onClick={() => api.openURL(info.url)}>
                    <Icon name="globe"/> {error ? t('update.downloadManually') : t('update.whatsNew')}
                </button>
                <button className="btn primary" disabled={applying}
                        onClick={() => (activeTunnels > 0 ? setConfirm(true) : apply())}>
                    <Icon name="refresh"/> {t('update.apply')}
                </button>
                {!applying && <button className="btn ghost" onClick={onDismiss}>{t('update.later')}</button>}
            </div>
            {confirm && (
                <ConfirmDialog
                    title={t('update.confirmTitle', {version: info.latest})}
                    confirmLabel={t('update.apply')}
                    confirmIcon="refresh"
                    danger={false}
                    onConfirm={apply}
                    onCancel={() => setConfirm(false)}
                >
                    <p>{t('update.confirmTunnels', {count: String(activeTunnels)})}</p>
                </ConfirmDialog>
            )}
        </div>
    )
}
