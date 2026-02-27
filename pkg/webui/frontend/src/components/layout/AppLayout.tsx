import { useEffect, useState } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { getToken, api } from '@/api/client';
import Sidebar from './Sidebar';

export default function AppLayout() {
  const navigate = useNavigate();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const token = getToken();
    if (!token) {
      navigate('/login', { replace: true });
      return;
    }

    // Check if the system has been initialized
    api
      .get<{ initialized: boolean }>('/api/auth/init-status')
      .then((data) => {
        if (!data.initialized) {
          navigate('/init', { replace: true });
        } else {
          setReady(true);
        }
      })
      .catch(() => {
        // If the init-status check fails (e.g. 401), the api client
        // will redirect to /login automatically. Otherwise just show the app.
        setReady(true);
      });
  }, [navigate]);

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <div className="animate-pulse text-muted-foreground text-sm">Loading\u2026</div>
      </div>
    );
  }

  return (
    <div className="h-screen flex bg-background font-sans text-foreground">
      <Sidebar />
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <main className="flex-1 overflow-auto custom-scrollbar p-6 md:p-8">
          <div className="max-w-6xl mx-auto animate-fade-in">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
