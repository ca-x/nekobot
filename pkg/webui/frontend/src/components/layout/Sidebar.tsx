import { cn } from '@/lib/utils';
import { t, getLanguage, setLanguage, type I18nLang } from '@/lib/i18n';
import { getTheme, toggleTheme, type Theme } from '@/lib/theme';
import { useUiStore } from '@/stores/ui.store';
import { clearToken } from '@/api/client';
import {
  Menu,
  X,
  MessageSquare,
  Terminal,
  Cpu,
  Boxes,
  Radio,
  Store,
  NotebookPen,
  Settings,
  Activity,
  Library,
  Clock3,
  ShieldCheck,
  Waypoints,
  Sun,
  Moon,
  LogOut,
  Languages,
  PanelLeftClose,
  PanelLeft,
} from 'lucide-react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useEffect, useRef, useState } from 'react';

const LANGUAGE_OPTIONS: { value: I18nLang; label: string }[] = [
  { value: 'en', label: 'EN' },
  { value: 'zh-CN', label: '\u4E2D\u6587' },
  { value: 'ja', label: '\u65E5\u672C\u8A9E' },
];

export default function Sidebar() {
  const navigate = useNavigate();
  const {
    sidebarOpen,
    mobileSidebarOpen,
    toggleSidebar,
    toggleMobileSidebar,
    setMobileSidebarOpen,
  } = useUiStore();
  const [currentTheme, setCurrentTheme] = useState<Theme>(getTheme());
  const [currentLang, setCurrentLang] = useState<I18nLang>(getLanguage());
  const mobileBackdropRef = useRef<HTMLDivElement | null>(null);
  const mobileAsideRef = useRef<HTMLElement | null>(null);

  const navItems = [
    { target: '/chat', label: t('tabChat'), icon: MessageSquare },
    { target: '/sessions', label: t('tabSessions'), icon: Library },
    { target: '/tools', label: t('tabTools'), icon: Terminal },
    { target: '/providers', label: t('tabProviders'), icon: Cpu },
    { target: '/models', label: t('tabModels'), icon: Boxes },
    { target: '/permission-rules', label: t('tabPermissionRules'), icon: ShieldCheck },
    { target: '/channels', label: t('tabChannels'), icon: Radio },
    { target: '/marketplace', label: t('tabMarketplace'), icon: Store },
    { target: '/prompts', label: t('tabPrompts'), icon: NotebookPen },
    { target: '/config', label: t('tabConfig'), icon: Settings },
    { target: '/harness/audit', label: t('tabHarnessAudit'), icon: ShieldCheck },
    { target: '/runtime-topology', label: t('tabRuntimeTopology'), icon: Waypoints },
    { target: '/cron', label: t('tabCron'), icon: Clock3 },
    { target: '/system', label: t('tabStatus'), icon: Activity },
  ];

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

  const handleLogout = () => {
    clearToken();
    navigate('/login');
  };

  const langLabel = LANGUAGE_OPTIONS.find((o) => o.value === currentLang)?.label ?? 'EN';
  const shellButtonClass =
    'flex h-10 w-10 items-center justify-center rounded-2xl border border-border/70 bg-card/90 text-muted-foreground shadow-sm transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2';
  const panelClass =
    'flex h-full min-h-0 flex-col border-r border-border/70 bg-card/88 text-foreground shadow-[0_24px_60px_-42px_rgba(15,23,42,0.55)] backdrop-blur-xl';

  const handleNavigate = () => {
    setMobileSidebarOpen(false);
  };

  useEffect(() => {
    const elements = [mobileBackdropRef.current, mobileAsideRef.current];
    for (const element of elements) {
      if (!element) {
        continue;
      }
      (element as HTMLElement & { inert?: boolean }).inert = !mobileSidebarOpen;
    }
  }, [mobileSidebarOpen]);

  const renderNav = () => (
    <nav className="custom-scrollbar flex-1 overflow-y-auto pr-1">
      <ul className="space-y-1">
        {navItems.map((item) => {
          const Icon = item.icon;
          return (
            <li key={item.target}>
              <NavLink
                to={item.target}
                onClick={handleNavigate}
                title={!sidebarOpen ? item.label : undefined}
                className={({ isActive }) =>
                  cn(
                    'group flex w-full items-center gap-3 rounded-2xl text-[13px] font-medium transition-colors duration-150',
                    sidebarOpen ? 'px-3 py-2.5' : 'justify-center px-0 py-2.5',
                    isActive
                      ? 'bg-accent text-accent-foreground font-semibold shadow-sm ring-1 ring-[hsl(var(--brand-200))/0.7]'
                      : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                  )
                }
              >
                {({ isActive }) => (
                  <>
                    <Icon
                      className={cn(
                        'h-4 w-4 shrink-0 transition-colors',
                        isActive
                          ? 'text-accent-foreground'
                          : 'text-muted-foreground group-hover:text-foreground',
                      )}
                    />
                    {sidebarOpen && <span className="truncate">{item.label}</span>}
                  </>
                )}
              </NavLink>
            </li>
          );
        })}
      </ul>
    </nav>
  );

  const renderActions = () => (
    <div className={cn('mt-4 shrink-0 space-y-1 border-t border-border/70 pt-4', !sidebarOpen && 'flex flex-col items-center space-y-2')}>
      <button
        onClick={handleLanguageSwitch}
        title={sidebarOpen ? `Language: ${langLabel}` : t('language')}
        aria-label={sidebarOpen ? `Language: ${langLabel}` : t('language')}
        className={cn(
          'flex w-full items-center gap-3 rounded-2xl text-[13px] font-medium text-muted-foreground transition-colors duration-150 hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
          sidebarOpen ? 'px-3 py-2.5' : 'justify-center px-0 py-2.5',
        )}
      >
        <Languages className="h-4 w-4 shrink-0" />
        {sidebarOpen && (
          <span className="flex-1 truncate text-left">{t('language')}</span>
        )}
        {sidebarOpen && (
          <span className="text-xs text-muted-foreground shrink-0">{langLabel}</span>
        )}
      </button>

      <button
        onClick={handleThemeToggle}
        title={currentTheme === 'dark' ? t('themeLight') : t('themeDark')}
        aria-label={currentTheme === 'dark' ? t('themeLight') : t('themeDark')}
        className={cn(
          'flex w-full items-center gap-3 rounded-2xl text-[13px] font-medium text-muted-foreground transition-colors duration-150 hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
          sidebarOpen ? 'px-3 py-2.5' : 'justify-center px-0 py-2.5',
        )}
      >
        {currentTheme === 'dark' ? (
          <Sun className="h-4 w-4 shrink-0" />
        ) : (
          <Moon className="h-4 w-4 shrink-0" />
        )}
        {sidebarOpen && (
          <span className="flex-1 truncate text-left">
            {currentTheme === 'dark' ? t('themeLight') : t('themeDark')}
          </span>
        )}
      </button>

      <button
        onClick={handleLogout}
        title={t('logout')}
        aria-label={t('logout')}
        className={cn(
          'flex w-full items-center gap-3 rounded-2xl text-[13px] font-medium text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
          sidebarOpen ? 'px-3 py-2.5' : 'justify-center px-0 py-2.5',
        )}
      >
        <LogOut className="h-4 w-4 shrink-0" />
        {sidebarOpen && <span className="flex-1 truncate text-left">{t('logout')}</span>}
      </button>
    </div>
  );

  return (
    <>
      <div className="fixed left-4 top-4 z-40 lg:hidden">
        <button
          onClick={toggleMobileSidebar}
          className={shellButtonClass}
          title={mobileSidebarOpen ? t('close') : t('open')}
          aria-label={mobileSidebarOpen ? t('close') : t('open')}
        >
          {mobileSidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </div>

      <div
        ref={mobileBackdropRef}
        aria-hidden={!mobileSidebarOpen}
        className={cn(
          'fixed inset-0 z-30 bg-background/70 backdrop-blur-sm transition-opacity lg:hidden',
          mobileSidebarOpen ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={() => setMobileSidebarOpen(false)}
      />

      <aside
        ref={mobileAsideRef}
        aria-hidden={!mobileSidebarOpen}
        className={cn(
          panelClass,
          'fixed inset-y-0 left-0 z-40 w-[84vw] max-w-[320px] px-4 py-5 transition-transform duration-200 lg:hidden',
          mobileSidebarOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="mb-6 flex items-center justify-between gap-3 px-1">
          <div className="flex min-w-0 items-center gap-3">
            <img
              src="/brand/nekobot-logo.png"
              alt="Nekobot"
              className="h-9 w-9 shrink-0 rounded-2xl object-cover shadow-sm"
            />
            <div className="min-w-0">
              <div className="truncate text-[15px] font-semibold tracking-tight">Nekobot</div>
              <div className="truncate text-xs text-muted-foreground">{t('appSubtitle')}</div>
            </div>
          </div>
          <button
            onClick={() => setMobileSidebarOpen(false)}
            className={cn(shellButtonClass, 'h-9 w-9 rounded-xl shadow-none')}
            title={t('close')}
            aria-label={t('close')}
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        {renderNav()}
        {renderActions()}
      </aside>

      <aside
        className={cn(
          panelClass,
          'hidden shrink-0 overflow-hidden transition-[width,padding] duration-200 lg:flex',
          sidebarOpen ? 'w-[232px] px-3 py-5' : 'w-[70px] px-2 py-5',
        )}
      >
        <div
          className={cn(
            'mb-6 w-full',
            sidebarOpen ? 'flex items-center justify-between' : 'flex flex-col items-center gap-2',
          )}
        >
          {sidebarOpen ? (
            <div className="flex min-w-0 items-center gap-2.5">
              <img
                src="/brand/nekobot-logo.png"
                alt="Nekobot"
                className="h-7 w-7 shrink-0 rounded-xl object-cover shadow-sm"
              />
              <span className="truncate text-[15px] font-semibold tracking-tight">Nekobot</span>
            </div>
          ) : (
            <img
              src="/brand/nekobot-logo.png"
              alt="Nekobot"
              className="h-9 w-9 rounded-2xl object-cover shadow-sm"
            />
          )}
          <button
            onClick={toggleSidebar}
            className={cn(
              'rounded-xl p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
              !sidebarOpen && 'mt-1',
            )}
            title={sidebarOpen ? t('sidebarCollapse') : t('sidebarExpand')}
            aria-label={sidebarOpen ? t('sidebarCollapse') : t('sidebarExpand')}
          >
            {sidebarOpen ? (
              <PanelLeftClose className="h-4 w-4" />
            ) : (
              <PanelLeft className="h-4 w-4" />
            )}
          </button>
        </div>
        {renderNav()}
        {renderActions()}
      </aside>
    </>
  );
}
