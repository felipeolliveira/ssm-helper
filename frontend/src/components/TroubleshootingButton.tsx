import {useState} from 'react'
import {api, Message, toMessage} from '../api'
import ConfirmDialog from './ConfirmDialog'
import {Icon} from './Icon'
import {useI18n} from '../i18n'

// Saves the troubleshooting file after explaining that it stays on this
// computer and should only go to support or people the user trusts.
export default function TroubleshootingButton({className, onError}: {
    className: string
    onError: (m: Message) => void
}) {
    const {t} = useI18n()
    const [confirm, setConfirm] = useState(false)
    const [saving, setSaving] = useState(false)

    const save = async () => {
        setConfirm(false)
        setSaving(true)
        try {
            await api.saveTroubleshooting()
        } catch (err) {
            onError(toMessage(err))
        }
        setSaving(false)
    }

    return (
        <>
            <button className={className} disabled={saving} title={t('troubleshooting.hint')} onClick={() => setConfirm(true)}>
                <Icon name="file"/> {saving ? t('troubleshooting.saving') : t('troubleshooting.save')}
            </button>
            {confirm && (
                <ConfirmDialog
                    title={t('troubleshooting.title')}
                    confirmLabel={t('troubleshooting.confirm')}
                    confirmIcon="save"
                    danger={false}
                    onConfirm={save}
                    onCancel={() => setConfirm(false)}
                >
                    <p>{t('troubleshooting.local')}</p>
                    <p>{t('troubleshooting.contents')}</p>
                    <p><strong>{t('troubleshooting.share')}</strong></p>
                </ConfirmDialog>
            )}
        </>
    )
}
