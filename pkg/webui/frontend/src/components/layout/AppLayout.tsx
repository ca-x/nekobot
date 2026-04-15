import { useEffect, useState } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { getToken, api } from '@/api/client';
import { t } from '@/lib/i18n';
import Sidebar from './Sidebar';

interface InitStatusResponse {
  initialized: boolean;
}

export default function AppLayout() {
  const navigate = useNavigate();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;

    fetch('/api/auth/init-status')
      .then(async (resp) => {
        if (!resp.ok) {
          throw new Error(`init status failed: ${resp.status}`);
        }
        return (await resp.json()) as InitStatusResponse;
      })
      .then((data) => {
        if (cancelled) {
          return;
        }
        if (!data.initialized) {
          navigate('/init', { replace: true });
          return;
        }

        const token = getToken();
        if (!token) {
          navigate('/login', { replace: true });
          return;
        }

        setReady(true);
      })
      .catch(() => {
        if (cancelled) {
          return;
        }

        const token = getToken();
        if (!token) {
          navigate('/login', { replace: true });
          return;
        }

        setReady(true);
      });

    return () => {
      cancelled = true;
    };
  }, [navigate]);

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <div className="animate-pulse text-muted-foreground text-sm">{t('systemLoading')}</div>
      </div>
    );
  }

  return (
    <div className="relative flex h-dvh overflow-hidden bg-background font-sans text-foreground">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,hsl(var(--brand-200))/0.34,transparent_26%),radial-gradient(circle_at_78%_12%,hsl(var(--brand-400))/0.16,transparent_18%),radial-gradient(circle_at_50%_100%,hsl(var(--brand-100))/0.4,transparent_38%),linear-gradient(180deg,hsl(var(--background)),hsl(var(--background))/0.96_26%,hsl(var(--background))/0.98)] dark:bg-[radial-gradient(circle_at_top_left,hsl(var(--brand-500))/0.24,transparent_22%),radial-gradient(circle_at_78%_12%,hsl(var(--brand-400))/0.14,transparent_16%),radial-gradient(circle_at_50%_100%,hsl(var(--brand-700))/0.18,transparent_32%),linear-gradient(180deg,hsl(var(--background)),hsl(var(--background))/0.95_24%,hsl(var(--background))/0.98)]" />
        <div className="absolute left-[12%] top-[-12%] h-[28rem] w-[28rem] rounded-full bg-[hsl(var(--brand-300))/0.18] blur-[130px] dark:bg-[hsl(var(--brand-500))/0.12]" />
        <div className="absolute bottom-[-16%] right-[8%] h-[24rem] w-[24rem] rounded-full bg-[hsl(var(--brand-200))/0.18] blur-[120px] dark:bg-[hsl(var(--brand-600))/0.12]" />
        <div className="absolute inset-0 backdrop-blur-[44px]" />
      </div>

      <Sidebar />
      <div className="relative z-10 flex min-w-0 flex-1 flex-col overflow-hidden">
        <main className="relative flex-1 overflow-auto custom-scrollbar px-3 pb-4 pt-16 sm:px-4 sm:pb-5 sm:pt-[4.5rem] lg:px-5 lg:pb-6 lg:pt-5 xl:px-6">
          <div className="pointer-events-none absolute inset-x-0 top-0 h-32 bg-gradient-to-b from-white/25 to-transparent dark:from-white/5" />
          <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-[hsl(var(--brand-300))/0.5] to-transparent dark:via-[hsl(var(--brand-500))/0.32]" />
          <div className="relative mx-auto w-full max-w-[1680px] animate-fade-in">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
