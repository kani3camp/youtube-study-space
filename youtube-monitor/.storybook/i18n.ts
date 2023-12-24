import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import Backend from 'i18next-http-backend'

i18n.use(initReactI18next)
    .use(Backend)
    .init({
        lng: 'ja',
        fallbackLng: 'ja',
        debug: true,
        defaultNS: 'common',
        ns: 'common',
        supportedLngs: ['ja', 'en', 'ko'],
        backend: {
            loadPath: '/locales/{{lng}}/{{ns}}.json',
        },
    })

export default i18n
