import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import AppLayout from './components/layout/AppLayout';
import { t } from '@/lib/i18n';

const LoginPage = lazy(() => import('./pages/LoginPage'));
const InitPage = lazy(() => import('./pages/InitPage'));
const ChatPage = lazy(() => import('./pages/ChatPage'));
const ProvidersPage = lazy(() => import('./pages/ProvidersPage'));
const ModelsPage = lazy(() => import('./pages/ModelsPage'));
const ChannelsPage = lazy(() => import('./pages/ChannelsPage'));
const DaemonPage = lazy(() => import('./pages/DaemonPage'));
const SystemPage = lazy(() => import('./pages/SystemPage'));

function Loading() {
  return (
    <div className="flex h-screen items-center justify-center">
      <div className="animate-pulse-soft text-muted-foreground">{t('loading')}</div>
    </div>
  );
}

export default function App() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/init" element={<InitPage />} />
        <Route element={<AppLayout />}>
          <Route path="/chat" element={<ChatPage />} />
          <Route path="/providers" element={<ProvidersPage />} />
          <Route path="/models" element={<ModelsPage />} />
          <Route path="/channels" element={<ChannelsPage />} />
          <Route path="/daemon" element={<DaemonPage />} />
          <Route path="/system" element={<SystemPage />} />
          <Route path="/" element={<Navigate to="/chat" replace />} />
          <Route path="*" element={<Navigate to="/chat" replace />} />
        </Route>
      </Routes>
    </Suspense>
  );
}
