import { type ReactNode, useState } from 'react';
import { Languages, Moon, Sun } from 'lucide-react';
import { getLanguage, setLanguage, type I18nLang, t } from '@/lib/i18n';
import { getTheme, toggleTheme, type Theme } from '@/lib/theme';

type AuthGradientVariant = 'login' | 'init';

interface AuthGradientShellProps {
  children: ReactNode;
  variant?: AuthGradientVariant;
}

export default function AuthGradientShell({
  children,
  variant = 'login',
}: AuthGradientShellProps) {
  const [currentTheme, setCurrentTheme] = useState<Theme>(getTheme());
  const [currentLang, setCurrentLang] = useState<I18nLang>(getLanguage());

  const handleThemeToggle = () => {
    const next = toggleTheme();
    setCurrentTheme(next);
  };

  const handleLanguageSwitch = () => {
    const langs: I18nLang[] = ['en', 'zh-CN', 'ja'];
    const idx = langs.indexOf(currentLang);
    const next = langs[(idx + 1) % langs.length];
    setLanguage(next);
    setCurrentLang(next);
    window.location.reload();
  };

  const langLabel = currentLang === 'zh-CN' ? '中文' : currentLang === 'ja' ? '日本語' : 'EN';
  const variantTone =
    variant === 'init'
      ? 'bg-[radial-gradient(circle_at_top,hsl(var(--brand-200))/0.32,transparent_28%),radial-gradient(circle_at_82%_16%,hsl(var(--brand-400))/0.18,transparent_24%),linear-gradient(180deg,hsl(var(--background))/0.18,transparent_35%,hsl(var(--background))/0.72_100%)] dark:bg-[radial-gradient(circle_at_top,hsl(var(--brand-500))/0.24,transparent_24%),radial-gradient(circle_at_82%_16%,hsl(var(--brand-400))/0.14,transparent_22%),linear-gradient(180deg,hsl(var(--background))/0.2,transparent_35%,hsl(var(--background))/0.8_100%)]'
      : 'bg-[radial-gradient(circle_at_top,hsl(var(--brand-200))/0.28,transparent_32%),radial-gradient(circle_at_80%_18%,hsl(var(--brand-400))/0.16,transparent_24%),linear-gradient(180deg,hsl(var(--background))/0.15,transparent_35%,hsl(var(--background))/0.68_100%)] dark:bg-[radial-gradient(circle_at_top,hsl(var(--brand-500))/0.22,transparent_26%),radial-gradient(circle_at_80%_18%,hsl(var(--brand-400))/0.12,transparent_22%),linear-gradient(180deg,hsl(var(--background))/0.18,transparent_35%,hsl(var(--background))/0.76_100%)]';

  return (
    <div className="relative isolate min-h-screen overflow-hidden bg-background">
      <div className="pointer-events-none absolute inset-0" aria-hidden="true">
        <div className={`absolute inset-0 ${variantTone}`} />
        <div className="absolute left-[12%] top-[-10%] h-[24rem] w-[24rem] rounded-full bg-[hsl(var(--brand-300))/0.18] blur-[120px] dark:bg-[hsl(var(--brand-500))/0.14]" />
        <div className="absolute bottom-[-14%] right-[8%] h-[22rem] w-[22rem] rounded-full bg-[hsl(var(--brand-200))/0.18] blur-[110px] dark:bg-[hsl(var(--brand-600))/0.12]" />
        <div className="absolute inset-0 backdrop-blur-[52px]" />
      </div>

      <div className="absolute right-4 top-4 z-20 flex items-center gap-2 sm:right-6 sm:top-6">
        <button
          type="button"
          onClick={handleLanguageSwitch}
          className="inline-flex h-10 items-center gap-2 rounded-2xl border border-border/70 bg-background/75 px-3 text-xs font-medium text-foreground shadow-sm backdrop-blur-xl transition-colors hover:bg-accent hover:text-accent-foreground"
          aria-label={t('language')}
          title={t('language')}
        >
          <Languages className="h-4 w-4" />
          <span>{langLabel}</span>
        </button>
        <button
          type="button"
          onClick={handleThemeToggle}
          className="inline-flex h-10 w-10 items-center justify-center rounded-2xl border border-border/70 bg-background/75 text-foreground shadow-sm backdrop-blur-xl transition-colors hover:bg-accent hover:text-accent-foreground"
          aria-label={currentTheme === 'dark' ? t('themeLight') : t('themeDark')}
          title={currentTheme === 'dark' ? t('themeLight') : t('themeDark')}
        >
          {currentTheme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </button>
      </div>

      <div className="relative z-10">{children}</div>
    </div>
  );
}
