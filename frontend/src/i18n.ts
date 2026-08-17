import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

/**
 * Internationalization setup, following the pattern used by Dify and Grafana:
 * react-i18next with one JSON resource file per language.
 *
 * Languages are AUTO-DISCOVERED from src/locales/*.json — adding a language is
 * a single-file contribution with no registration step (see
 * src/locales/README.md). Each file carries its own display name under
 * `meta.name`, shown natively in the language switcher.
 */

interface LocaleModule {
  default: { meta?: { name?: string } } & Record<string, unknown>
}

const localeModules = import.meta.glob<LocaleModule>('./locales/*.json', {
  eager: true,
})

export interface Language {
  /** BCP 47-ish code taken from the filename, e.g. "en", "ko", "zh-Hans". */
  code: string
  /** Native display name from the file's `meta.name`, e.g. "한국어". */
  name: string
}

const resources: Record<string, { translation: LocaleModule['default'] }> = {}
const languages: Language[] = []

for (const [path, module] of Object.entries(localeModules)) {
  const code = path.replace('./locales/', '').replace('.json', '')
  resources[code] = { translation: module.default }
  languages.push({ code, name: module.default.meta?.name ?? code })
}

/** Every discovered language, sorted by code for a stable switcher order. */
export const SUPPORTED_LANGUAGES: readonly Language[] = languages.sort((a, b) =>
  a.code.localeCompare(b.code),
)

export const FALLBACK_LANGUAGE = 'en'

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: FALLBACK_LANGUAGE,
    supportedLngs: SUPPORTED_LANGUAGES.map((lang) => lang.code),
    // "en-US" from the browser should match our "en" resource.
    nonExplicitSupportedLngs: true,
    detection: {
      // A choice made in the UI wins over the browser locale.
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      lookupLocalStorage: 'starlens.lang',
    },
    interpolation: {
      // React already escapes rendered strings.
      escapeValue: false,
    },
    // Resources are bundled eagerly, so init is synchronous — components can
    // translate on first render with no loading state.
    initAsync: false,
  })

// Keep the document language in sync for accessibility and font selection.
i18n.on('languageChanged', (lng) => {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = lng
  }
})
if (typeof document !== 'undefined' && i18n.resolvedLanguage) {
  document.documentElement.lang = i18n.resolvedLanguage
}

export default i18n
