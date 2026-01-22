import i18n, { BackendModule, FallbackLng, FallbackLngObjList } from "i18next";
import { orderBy } from "lodash-es";
import { initReactI18next } from "react-i18next";
import { findNearestMatchedLanguage } from "./utils/i18n";
// Import English translations directly to ensure they're always available as fallback
import enTranslations from "./locales/en.json";

export const locales = orderBy([
  "ar",
  "cs",
  "de",
  "en",
  "en-GB",
  "es",
  "fa",
  "fr",
  "hi",
  "hr",
  "hu",
  "id",
  "it",
  "ja",
  "ka-GE",
  "ko",
  "mr",
  "nb",
  "nl",
  "pl",
  "pt-PT",
  "pt-BR",
  "ru",
  "sl",
  "sv",
  "th",
  "tr",
  "uk",
  "vi",
  "zh-Hans",
  "zh-Hant",
]);

const fallbacks = {
  "zh-HK": ["zh-Hant", "en"],
  "zh-TW": ["zh-Hant", "en"],
  zh: ["zh-Hans", "en"],
} as FallbackLngObjList;

const LazyImportPlugin: BackendModule = {
  type: "backend",
  init: function () {},
  read: function (language, namespace, callback) {
    const matchedLanguage = findNearestMatchedLanguage(language);
    import(`./locales/${matchedLanguage}.json`)
      .then((module: any) => {
        // Handle both wrapped ({ default: ... }) and unwrapped JSON imports
        // Vite wraps JSON in a module object with default export
        const translations = module.default || module;
        callback(null, translations);
      })
      .catch((error) => {
        console.error(`Failed to load translations for ${matchedLanguage}:`, error);
        // Try loading English as fallback
        if (matchedLanguage !== "en") {
          import("./locales/en.json")
            .then((module: any) => {
              const translations = module.default || module;
              callback(null, translations);
            })
            .catch((fallbackError) => {
              console.error("Failed to load English fallback:", fallbackError);
              callback(fallbackError, null);
            });
        } else {
          callback(error, null);
        }
      });
  },
};

i18n
  .use(LazyImportPlugin)
  .use(initReactI18next)
  .init({
    // Explicit namespace configuration
    ns: ["translation"],
    defaultNS: "translation",
    fallbackNS: "translation",
    // Language detection
    lng: "en", // Set default language explicitly
    detection: {
      order: ["navigator"],
    },
    fallbackLng: {
      ...fallbacks,
      ...{ default: ["en"] },
    } as FallbackLng,
    // Ensure missing keys fall back to English instead of showing the key
    returnEmptyString: false,
    returnNull: false,
    // Don't show key when translation is missing - use fallback
    saveMissing: false,
    // Preload English translations so they're always available as fallback
    resources: {
      en: {
        translation: enTranslations,
      },
    },
    // Allow backend to still load other languages dynamically
    partialBundledLanguages: true,
    // Interpolation settings
    interpolation: {
      escapeValue: false, // React already escapes
    },
  });

export default i18n;
export type TLocale = (typeof locales)[number];
