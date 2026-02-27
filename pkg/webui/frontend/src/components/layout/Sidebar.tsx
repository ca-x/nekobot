import { cn } from '@/lib/utils';
import { t, getLanguage, setLanguage, type I18nLang } from '@/lib/i18n';
import { getTheme, toggleTheme, type Theme } from '@/lib/theme';
import { useUiStore } from '@/stores/ui.store';
import { clearToken } from '@/api/client';
import {
  MessageSquare,
  Terminal,
  Cpu,
  Radio,
  Settings,
  Activity,
  Sun,
  Moon,
  LogOut,
  Languages,
  PanelLeftClose,
  PanelLeft,
} from 'lucide-react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useState } from 'react';

const LANGUAGE_OPTIONS: { value: I18nLang; label: string }[] = [
  { value: 'en', label: 'EN' },
  { value: 'zh-CN', label: '\u4E2D\u6587' },
  { value: 'ja', label: '\u65E5\u672C\u8A9E' },
];

export default function Sidebar() {
  const navigate = useNavigate();
  const { sidebarOpen, toggleSidebar } = useUiStore();
  const [currentTheme, setCurrentTheme] = useState<Theme>(getTheme());
  const [currentLang, setCurrentLang] = useState<I18nLang>(getLanguage());

  const navItems = [
    { target: '/chat', label: t('tabChat'), icon: MessageSquare },
    { target: '/tools', label: t('tabTools'), icon: Terminal },
    { target: '/providers', label: t('tabProviders'), icon: Cpu },
    { target: '/channels', label: t('tabChannels'), icon: Radio },
    { target: '/config', label: t('tabConfig'), icon: Settings },
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

  return (
    <aside
      className={cn(
        'shrink-0 flex flex-col h-full bg-secondary transition-[width,padding] duration-200 overflow-hidden',
        sidebarOpen ? 'w-[220px] py-5 px-3' : 'w-[56px] py-5 px-1.5',
      )}
    >
      {/* Logo + Collapse Toggle */}
      <div className={cn('flex items-center mb-6', sidebarOpen ? 'px-2 justify-between' : 'justify-center')}>
        {sidebarOpen && (
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-lg leading-none" role="img" aria-label="cat">
              🐱
            </span>
            <span className="text-[15px] font-semibold text-foreground tracking-tight truncate">
              Nekobot
            </span>
          </div>
        )}
        <button
          onClick={toggleSidebar}
          className="p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          title={sidebarOpen ? 'Collapse sidebar' : 'Expand sidebar'}
        >
          {sidebarOpen ? (
            <PanelLeftClose className="h-4 w-4" />
          ) : (
            <PanelLeft className="h-4 w-4" />
          )}
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1">
        <ul className="space-y-0.5">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <li key={item.target}>
                <NavLink
                  to={item.target}
                  title={!sidebarOpen ? item.label : undefined}
                  className={({ isActive }) =>
                    cn(
                      'group w-full flex items-center gap-2.5 rounded-xl text-[13px] font-medium transition-colors duration-150',
                      sidebarOpen ? 'px-3 py-2' : 'px-0 py-2 justify-center',
                      isActive
                        ? 'bg-accent text-accent-foreground font-semibold'
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

      {/* Bottom Actions */}
      <div className={cn('pt-3 border-t border-border mt-3 space-y-0.5', !sidebarOpen && 'flex flex-col items-center')}>
        {/* Language Selector */}
        <button
          onClick={handleLanguageSwitch}
          title={`Language: ${langLabel}`}
          className={cn(
            'w-full flex items-center gap-2.5 rounded-xl text-[13px] font-medium transition-colors duration-150 text-muted-foreground hover:bg-muted hover:text-foreground',
            sidebarOpen ? 'px-3 py-2' : 'px-0 py-2 justify-center',
          )}
        >
          <Languages className="h-4 w-4 shrink-0" />
          {sidebarOpen && (
            <>
              <span className="flex-1 text-left truncate">{t('language') !== 'language' ? t('language') : 'Language'}</span>
              <span className="text-xs text-muted-foreground">{langLabel}</span>
            </>
          )}
        </button>

        {/* Theme Toggle */}
        <button
          onClick={handleThemeToggle}
          title={currentTheme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
          className={cn(
            'w-full flex items-center gap-2.5 rounded-xl text-[13px] font-medium transition-colors duration-150 text-muted-foreground hover:bg-muted hover:text-foreground',
            sidebarOpen ? 'px-3 py-2' : 'px-0 py-2 justify-center',
          )}
        >
          {currentTheme === 'dark' ? (
            <Sun className="h-4 w-4 shrink-0" />
          ) : (
            <Moon className="h-4 w-4 shrink-0" />
          )}
          {sidebarOpen && (
            <span className="flex-1 text-left truncate">
              {currentTheme === 'dark' ? 'Light Mode' : 'Dark Mode'}
            </span>
          )}
        </button>

        {/* Logout */}
        <button
          onClick={handleLogout}
          title={t('logout')}
          className={cn(
            'w-full flex items-center gap-2.5 rounded-xl text-[13px] font-medium transition-colors duration-150 text-muted-foreground hover:bg-destructive/10 hover:text-destructive',
            sidebarOpen ? 'px-3 py-2' : 'px-0 py-2 justify-center',
          )}
        >
          <LogOut className="h-4 w-4 shrink-0" />
          {sidebarOpen && <span className="flex-1 text-left truncate">{t('logout')}</span>}
        </button>
      </div>
    </aside>
  );
}
