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
    <div className="flex h-dvh bg-background font-sans text-foreground">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <main className="flex-1 overflow-auto custom-scrollbar px-3 pt-16 pb-4 sm:px-4 sm:pt-[4.5rem] sm:pb-5 lg:px-5 lg:pt-5 lg:pb-6 xl:px-6">
          <div className="w-full animate-fade-in">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
