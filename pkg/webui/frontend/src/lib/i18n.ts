export type I18nLang = 'en' | 'zh-CN' | 'ja';

const STORAGE_KEY = 'nekobot_lang';

let currentLang: I18nLang = 'en';
let dict: Record<string, string> = {};
let listeners: Array<() => void> = [];

export function getLanguage(): I18nLang {
  if (typeof window === 'undefined') return 'en';
  const saved = localStorage.getItem(STORAGE_KEY) as I18nLang | null;
  if (saved && ['en', 'zh-CN', 'ja'].includes(saved)) return saved;
  const nav = navigator.language;
  if (nav.startsWith('zh')) return 'zh-CN';
  if (nav.startsWith('ja')) return 'ja';
  return 'en';
}

export function setLanguage(lang: I18nLang) {
  currentLang = lang;
  localStorage.setItem(STORAGE_KEY, lang);
  loadDict(lang).then(() => {
    listeners.forEach((fn) => fn());
  });
}

export async function loadDict(lang?: I18nLang) {
  const target = lang ?? currentLang;
  try {
    const resp = await fetch(`/i18n/${target}.json`);
    if (resp.ok) {
      dict = await resp.json();
    }
  } catch {
    // fallback: keep current dict
  }
}

export function t(key: string, ...args: (string | number)[]): string {
  let val = dict[key] ?? key;
  args.forEach((arg, i) => {
    val = val.replace(`{${i}}`, String(arg));
  });
  return val;
}

export function subscribeI18n(fn: () => void): () => void {
  listeners.push(fn);
  return () => {
    listeners = listeners.filter((l) => l !== fn);
  };
}

export async function initI18n() {
  currentLang = getLanguage();
  await loadDict(currentLang);
}
