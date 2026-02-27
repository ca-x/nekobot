import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import AppLayout from './components/layout/AppLayout';

const LoginPage = lazy(() => import('./pages/LoginPage'));
const InitPage = lazy(() => import('./pages/InitPage'));
const ChatPage = lazy(() => import('./pages/ChatPage'));
const ToolSessionsPage = lazy(() => import('./pages/ToolSessionsPage'));
const ProvidersPage = lazy(() => import('./pages/ProvidersPage'));
const ChannelsPage = lazy(() => import('./pages/ChannelsPage'));
const ConfigPage = lazy(() => import('./pages/ConfigPage'));
const SystemPage = lazy(() => import('./pages/SystemPage'));

function Loading() {
  return (
    <div className="flex h-screen items-center justify-center">
      <div className="animate-pulse-soft text-muted-foreground">Loading...</div>
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
          <Route path="/tools" element={<ToolSessionsPage />} />
          <Route path="/providers" element={<ProvidersPage />} />
          <Route path="/channels" element={<ChannelsPage />} />
          <Route path="/config" element={<ConfigPage />} />
          <Route path="/system" element={<SystemPage />} />
          <Route path="/" element={<Navigate to="/chat" replace />} />
        </Route>
      </Routes>
    </Suspense>
  );
}
