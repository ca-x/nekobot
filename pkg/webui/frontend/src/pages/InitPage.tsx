import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, setToken } from '@/api/client';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';

interface InitResponse {
  token: string;
}

export default function InitPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const data = await api.post<InitResponse>('/api/auth/init', {
        username,
        password,
      });
      setToken(data.token);
      navigate('/chat', { replace: true });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Initialization failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        <div className="rounded-2xl border border-border bg-card p-8 shadow-card">
          {/* Logo */}
          <div className="flex items-center justify-center gap-2 mb-2">
            <span className="text-2xl" role="img" aria-label="cat">
              🐱
            </span>
            <span className="text-xl font-semibold text-foreground tracking-tight">
              Nekobot
            </span>
          </div>
          <p className="text-center text-sm text-muted-foreground mb-1">
            {t('initTitle')}
          </p>
          <p className="text-center text-xs text-muted-foreground mb-6">
            {t('firstRunHint')}
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label
                htmlFor="username"
                className="block text-sm font-medium text-foreground mb-1.5"
              >
                {t('username')}
              </label>
              <input
                id="username"
                type="text"
                autoComplete="username"
                required
                spellCheck={false}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                placeholder={t('username')}
              />
            </div>

            <div>
              <label
                htmlFor="password"
                className="block text-sm font-medium text-foreground mb-1.5"
              >
                {t('password')}
              </label>
              <input
                id="password"
                type="password"
                autoComplete="new-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                placeholder={t('password')}
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full rounded-xl bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:opacity-90 disabled:opacity-50 transition-opacity"
            >
              {loading ? '\u2026' : t('initialize')}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
