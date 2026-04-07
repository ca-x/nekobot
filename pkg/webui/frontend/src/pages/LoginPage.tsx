import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { api, setToken } from '@/api/client';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';
import { PasswordInput } from '@/components/ui/password-input';
import { Button } from '@/components/ui/button';

interface LoginResponse {
  token: string;
}

interface InitStatusResponse {
  initialized: boolean;
}

export default function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

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
        if (cancelled || data.initialized) {
          return;
        }
        navigate('/init', { replace: true });
      })
      .catch(() => {
        // Keep login accessible if init status is temporarily unavailable.
      });

    return () => {
      cancelled = true;
    };
  }, [navigate]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const data = await api.post<LoginResponse>('/api/auth/login', {
        username,
        password,
      });
      setToken(data.token);
      navigate('/chat', { replace: true });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('loginFailed'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        <div className="rounded-2xl border border-border bg-card p-8 shadow-card">
          {/* Logo */}
          <div className="mb-2 flex items-center justify-center gap-3">
            <img
              src="/brand/nekobot-logo.png"
              alt="Nekobot"
              className="h-10 w-10 rounded-xl object-cover shadow-sm"
            />
            <span className="text-xl font-semibold text-foreground tracking-tight">
              Nekobot
            </span>
          </div>
          <p className="text-center text-sm text-muted-foreground mb-6">
            {t('loginHint')}
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label
                htmlFor="username"
                className="block text-sm font-medium text-foreground mb-1.5"
              >
                {t('username')}
                <span className="ml-1 text-destructive">*</span>
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
                aria-label={t('username')}
              />
            </div>

            <PasswordInput
              id="password"
              label={t('password')}
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={t('password')}
            />

            <Button
              type="submit"
              disabled={loading || !username.trim() || !password.trim()}
              className="w-full"
            >
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('login')}
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}
