import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { api, setToken } from '@/api/client';
import { t } from '@/lib/i18n';
import { toast } from '@/lib/notify';
import { PasswordInput } from '@/components/ui/password-input';
import { Button } from '@/components/ui/button';
import AuthGradientShell from '@/components/layout/AuthGradientShell';

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
    <AuthGradientShell variant="login">
      <div className="flex min-h-screen items-center justify-center px-4 py-8 sm:px-6 lg:px-8">
        <div className="grid w-full max-w-5xl gap-6 lg:grid-cols-[1.05fr_440px]">
          <section className="hidden flex-col justify-between rounded-[2rem] border border-white/45 bg-white/40 p-8 text-foreground shadow-[0_32px_120px_-48px_rgba(15,23,42,0.45)] backdrop-blur-2xl dark:border-white/10 dark:bg-slate-950/40 lg:flex">
            <div>
              <div className="inline-flex items-center gap-3 rounded-full border border-border/70 bg-background/80 px-4 py-2 text-xs font-medium uppercase tracking-[0.22em] text-foreground/75">
                {t('loginHeroBadge')}
              </div>
              <div className="mt-8 flex items-center gap-4">
                <img
                  src="/brand/nekobot-logo.png"
                  alt="Nekobot"
                  className="h-14 w-14 rounded-2xl object-cover shadow-sm ring-1 ring-black/5 dark:ring-white/10"
                />
                <div>
                  <h1 className="text-3xl font-semibold tracking-tight text-foreground">
                    Nekobot
                  </h1>
                  <p className="mt-1 text-sm text-foreground/70">
                    {t('loginHeroSubtitle')}
                  </p>
                </div>
              </div>
              <p className="mt-8 max-w-xl text-sm leading-7 text-foreground/75">
                {t('loginHeroDescription')}
              </p>
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              {[
                [t('loginHeroMetricSessionsTitle'), t('loginHeroMetricSessionsDesc')],
                [t('loginHeroMetricProvidersTitle'), t('loginHeroMetricProvidersDesc')],
                [t('loginHeroMetricRuntimeTitle'), t('loginHeroMetricRuntimeDesc')],
              ].map(([title, desc]) => (
                <div
                  key={title}
                  className="rounded-2xl border border-border/60 bg-background/75 p-4 backdrop-blur-xl"
                >
                  <div className="text-sm font-semibold text-foreground">{title}</div>
                  <div className="mt-1 text-xs leading-5 text-foreground/70">{desc}</div>
                </div>
              ))}
            </div>
          </section>

          <section className="w-full">
            <div className="rounded-[2rem] border border-white/60 bg-white/78 p-6 shadow-[0_28px_120px_-56px_rgba(15,23,42,0.55)] backdrop-blur-2xl dark:border-white/10 dark:bg-[hsl(var(--card))/0.78] sm:p-8">
              <div className="mb-6 space-y-3 text-center">
                <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl border border-border/70 bg-background/80 shadow-sm">
                  <img
                    src="/brand/nekobot-logo.png"
                    alt="Nekobot"
                    className="h-10 w-10 rounded-xl object-cover"
                  />
                </div>
                <div>
                  <h2 className="text-2xl font-semibold tracking-tight text-foreground">
                    {t('loginHeroTitle')}
                  </h2>
                  <p className="mt-1 text-sm text-muted-foreground">{t('loginHint')}</p>
                </div>
              </div>

              <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                  <label
                    htmlFor="username"
                    className="mb-1.5 block text-sm font-medium text-foreground"
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
                    className="w-full rounded-2xl border border-border/80 bg-background/85 px-4 py-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
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
                  className="h-11 w-full rounded-2xl"
                >
                  {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  {t('login')}
                </Button>
              </form>
            </div>
          </section>
        </div>
      </div>
    </AuthGradientShell>
  );
}
