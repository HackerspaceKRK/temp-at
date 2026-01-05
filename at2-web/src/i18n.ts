import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import pl from './locales/pl.json';

i18n
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
        fallbackLng: 'en',
        detection: {
            order: ['localStorage', 'htmlTag'],
            caches: ['localStorage'],
        },
        lng: localStorage.getItem('i18nextLng') || 'pl',
        debug: false,
        interpolation: {
            escapeValue: false,
        },
        resources: {
            pl: {
                translation: pl
            }
        }
    });

export default i18n;
