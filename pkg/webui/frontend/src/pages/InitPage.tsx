import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { api, setToken } from '@/api/client';
import { t } from '@/lib/i18n';
import { toast } from '@/lib/notify';
import { PasswordInput } from '@/components/ui/password-input';
import { Button } from '@/components/ui/button';
import AuthGradientShell from '@/components/layout/AuthGradientShell';

interface InitResponse {
  token: string;
  restart_required?: boolean;
  restart_sections?: string[];
}

interface InitStatusResponse {
  initialized: boolean;
  bootstrap: {
    config_path: string;
    db_dir: string;
    workspace: string;
    logger: {
      level: string;
    };
    gateway: {
      host: string;
      port: number;
    };
    webui: {
      enabled: boolean;
      port: number;
      public_base_url: string;
    };
    webhook: {
      enabled: boolean;
      path: string;
    };
    workspace_status: {
      path: string;
      bootstrapped: boolean;
      missing_bootstrap: string[];
    };
  };
}

export default function InitPage() {
  const navigate = useNavigate();
  const [bootstrapping, setBootstrapping] = useState(true);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [configPath, setConfigPath] = useState('');
  const [dbDir, setDBDir] = useState('');
  const [workspace, setWorkspace] = useState('');
  const [loggerLevel, setLoggerLevel] = useState('info');
  const [gatewayHost, setGatewayHost] = useState('0.0.0.0');
  const [gatewayPort, setGatewayPort] = useState('18790');
  const [webuiEnabled, setWebuiEnabled] = useState(true);
  const [webuiPort, setWebuiPort] = useState('0');
  const [publicBaseURL, setPublicBaseURL] = useState('');
  const [webhookPath, setWebhookPath] = useState('/api/webhooks/agent');
  const [workspaceBootstrapped, setWorkspaceBootstrapped] = useState(false);
  const [missingBootstrap, setMissingBootstrap] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [repairing, setRepairing] = useState(false);
  const [submitStatus, setSubmitStatus] = useState<'idle' | 'submitting' | 'restart_required'>('idle');
  const [restartSections, setRestartSections] = useState<string[]>([]);

  useEffect(() => {
    let cancelled = false;

    async function loadInitStatus() {
      try {
        const data = await api.get<InitStatusResponse>('/api/auth/init-status');
        if (cancelled) {
          return;
        }
        if (data.initialized) {
          navigate('/login', { replace: true });
          return;
        }
        setConfigPath(data.bootstrap.config_path);
        setDBDir(data.bootstrap.db_dir);
        setWorkspace(data.bootstrap.workspace);
        setLoggerLevel(data.bootstrap.logger.level || 'info');
        setGatewayHost(data.bootstrap.gateway.host || '0.0.0.0');
        setGatewayPort(String(data.bootstrap.gateway.port || 18790));
        setWebuiEnabled(Boolean(data.bootstrap.webui.enabled));
        setWebuiPort(String(data.bootstrap.webui.port ?? 0));
        setPublicBaseURL(data.bootstrap.webui.public_base_url || '');
        setWebhookPath(data.bootstrap.webhook?.path || '/api/webhooks/agent');
        setWorkspaceBootstrapped(Boolean(data.bootstrap.workspace_status?.bootstrapped));
        setMissingBootstrap(data.bootstrap.workspace_status?.missing_bootstrap ?? []);
      } catch (err) {
        if (!cancelled) {
          toast.error(err instanceof Error ? err.message : t('initLoadFailed'));
        }
      } finally {
        if (!cancelled) {
          setBootstrapping(false);
        }
      }
    }

    void loadInitStatus();
    return () => {
      cancelled = true;
    };
  }, [navigate]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setSubmitStatus('submitting');
    setRestartSections([]);
    try {
      const data = await api.post<InitResponse>('/api/auth/init', {
        username,
        password,
        bootstrap: {
          logger: {
            level: loggerLevel,
          },
          gateway: {
            host: gatewayHost,
            port: Number(gatewayPort),
          },
          webui: {
            enabled: webuiEnabled,
            port: Number(webuiPort),
            public_base_url: publicBaseURL.trim(),
          },
        },
      });
      setToken(data.token);
      if (data.restart_required) {
        const sections = (data.restart_sections ?? []).join(', ');
        setRestartSections(data.restart_sections ?? []);
        setSubmitStatus('restart_required');
        toast.info(t('initRestartRequired', sections));
        sessionStorage.setItem('nekobot_init_handoff', JSON.stringify({ restartSections: data.restart_sections ?? [] }));
      } else {
        setSubmitStatus('idle');
        sessionStorage.setItem('nekobot_init_handoff', JSON.stringify({ restartSections: [] }));
      }
      navigate('/chat', { replace: true });
    } catch (err) {
      setSubmitStatus('idle');
      toast.error(err instanceof Error ? err.message : t('initFailed'));
    } finally {
      setLoading(false);
    }
  };

  const handleRepairWorkspace = async () => {
    setRepairing(true);
    try {
      const data = await api.post<{ workspace_status: { bootstrapped: boolean; missing_bootstrap: string[] } }>(
        '/api/auth/init/repair-workspace',
        {},
      );
      setWorkspaceBootstrapped(Boolean(data.workspace_status?.bootstrapped));
      setMissingBootstrap(data.workspace_status?.missing_bootstrap ?? []);
      toast.success(t('initWorkspaceRepaired'));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('initWorkspaceRepairFailed'));
    } finally {
      setRepairing(false);
    }
  };

  if (bootstrapping) {
    return (
      <AuthGradientShell variant="init">
        <div className="flex min-h-screen items-center justify-center p-4">
          <div className="rounded-2xl border border-white/50 bg-white/70 px-5 py-4 text-sm text-muted-foreground shadow-card backdrop-blur-xl dark:border-white/10 dark:bg-[hsl(var(--card))/0.78]">
            {t('checkingInit')}
          </div>
        </div>
      </AuthGradientShell>
    );
  }

  return (
    <AuthGradientShell variant="init">
      <div className="flex min-h-screen items-center justify-center px-4 py-8 sm:px-6 lg:px-8">
        <div className="w-full max-w-6xl rounded-[2rem] border border-white/60 bg-white/76 p-6 shadow-[0_28px_120px_-56px_rgba(15,23,42,0.58)] backdrop-blur-2xl dark:border-white/10 dark:bg-[hsl(var(--card))/0.78] sm:p-8">
          <div className="mb-8 flex flex-col gap-5 border-b border-border/70 pb-6 lg:flex-row lg:items-end lg:justify-between">
            <div className="min-w-0">
              <div className="inline-flex items-center gap-3 rounded-full border border-border/70 bg-background/70 px-4 py-2 text-xs font-medium uppercase tracking-[0.22em] text-muted-foreground">
                {t('initHeroBadge')}
              </div>
              <div className="mt-5 flex items-center gap-4">
                <img
                  src="/brand/nekobot-logo.png"
                  alt="Nekobot"
                  className="h-12 w-12 rounded-2xl object-cover shadow-sm ring-1 ring-black/5 dark:ring-white/10"
                />
                <div>
                  <h1 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
                    {t('initTitle')}
                  </h1>
                  <p className="mt-1 text-sm text-muted-foreground">{t('firstRunHint')}</p>
                </div>
              </div>
            </div>

            <div className="grid gap-3 text-left sm:grid-cols-3">
              {[
                [t('initHeroAdminTitle'), t('initHeroAdminDesc')],
                [t('initHeroRuntimeTitle'), t('initHeroRuntimeDesc')],
                [t('initHeroAccessTitle'), t('initHeroAccessDesc')],
              ].map(([title, desc]) => (
                <div
                  key={title}
                  className="rounded-2xl border border-border/60 bg-background/60 px-4 py-3 backdrop-blur-xl"
                >
                  <div className="text-sm font-semibold text-foreground">{title}</div>
                  <div className="mt-1 text-xs leading-5 text-muted-foreground">{desc}</div>
                </div>
              ))}
            </div>
          </div>

          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="grid gap-6 lg:grid-cols-2">
              <div className="space-y-4 rounded-[1.5rem] border border-border/70 bg-background/62 p-5 backdrop-blur-xl">
                <div>
                  <div className="text-sm font-semibold text-foreground">{t('initAdminTitle')}</div>
                  <p className="mt-1 text-xs text-muted-foreground">{t('initAdminDesc')}</p>
                </div>

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
                    className="w-full rounded-2xl border border-border/80 bg-background/85 px-4 py-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                    placeholder={t('username')}
                  />
                </div>

                <PasswordInput
                  id="password"
                  label={t('password')}
                  required
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t('password')}
                  helperText={t('initPasswordHint')}
                />

                <div className="rounded-2xl border border-border/70 bg-muted/35 px-4 py-4 text-xs text-muted-foreground">
                  {submitStatus === 'submitting' ? (
                    <div className="flex items-start gap-2">
                      <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin" />
                      <div>
                        <div className="font-medium text-foreground">{t('initSubmittingTitle')}</div>
                        <div>{t('initSubmittingDescription')}</div>
                      </div>
                    </div>
                  ) : submitStatus === 'restart_required' ? (
                    <div>
                      <div className="font-medium text-foreground">{t('initRestartCardTitle')}</div>
                      <div>{t('initRestartRequired', restartSections.join(', '))}</div>
                    </div>
                  ) : (
                    <div>
                      <div className="font-medium text-foreground">{t('initNextStepTitle')}</div>
                      <div>{t('initNextStepDescription')}</div>
                    </div>
                  )}
                </div>
              </div>

              <div className="space-y-4 rounded-[1.5rem] border border-border/70 bg-background/62 p-5 backdrop-blur-xl">
                <div>
                  <div className="text-sm font-semibold text-foreground">{t('initBootstrapTitle')}</div>
                  <p className="mt-1 text-xs text-muted-foreground">{t('initBootstrapDesc')}</p>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      {t('initConfigPath')}
                    </label>
                    <div className="w-full rounded-2xl border border-border bg-muted px-4 py-3 text-sm text-muted-foreground break-all">
                      {configPath}
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      {t('initDBDir')}
                    </label>
                    <div className="w-full rounded-2xl border border-border bg-muted px-4 py-3 text-sm text-muted-foreground break-all">
                      {dbDir}
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      {t('initWorkspace')}
                    </label>
                    <div className="w-full rounded-2xl border border-border bg-muted px-4 py-3 text-sm text-muted-foreground break-all">
                      {workspace}
                    </div>
                  </div>

                  <div className="md:col-span-2 rounded-2xl border border-border/70 bg-muted/35 px-4 py-4 text-xs text-muted-foreground">
                    <div className="font-medium text-foreground mb-1">{t('initWorkspaceBootstrapTitle')}</div>
                    {workspaceBootstrapped
                      ? t('initWorkspaceBootstrapped')
                      : missingBootstrap.length > 0
                        ? t('initWorkspaceBootstrapMissing', missingBootstrap.join(', '))
                        : t('initWorkspaceNotBootstrapped')}
                    {!workspaceBootstrapped ? (
                      <div className="mt-3">
                        <Button type="button" variant="outline" onClick={handleRepairWorkspace} disabled={repairing}>
                          {repairing ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                          {t('initRepairWorkspace')}
                        </Button>
                      </div>
                    ) : null}
                  </div>

                  <div className="md:col-span-2 rounded-2xl border border-border/70 bg-muted/35 px-4 py-4 text-xs text-muted-foreground">
                    <div className="font-medium text-foreground mb-1">{t('initWebhookPathTitle')}</div>
                    <div>{webhookPath}</div>
                  </div>

                  <div>
                    <label
                      htmlFor="logger-level"
                      className="block text-sm font-medium text-foreground mb-1.5"
                    >
                      {t('initLoggerLevel')}
                    </label>
                    <select
                      id="logger-level"
                      value={loggerLevel}
                      onChange={(e) => setLoggerLevel(e.target.value)}
                      className="w-full rounded-2xl border border-border/80 bg-background/85 px-4 py-3 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
                    >
                      {['debug', 'info', 'warn', 'error'].map((level) => (
                        <option key={level} value={level}>
                          {level}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label
                      htmlFor="gateway-host"
                      className="block text-sm font-medium text-foreground mb-1.5"
                    >
                      {t('initGatewayHost')}
                    </label>
                    <input
                      id="gateway-host"
                      type="text"
                      value={gatewayHost}
                      onChange={(e) => setGatewayHost(e.target.value)}
                      className="w-full rounded-2xl border border-border/80 bg-background/85 px-4 py-3 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                    />
                  </div>

                  <div>
                    <label
                      htmlFor="gateway-port"
                      className="block text-sm font-medium text-foreground mb-1.5"
                    >
                      {t('initGatewayPort')}
                    </label>
                    <input
                      id="gateway-port"
                      type="number"
                      min="1"
                      value={gatewayPort}
                      onChange={(e) => setGatewayPort(e.target.value)}
                      className="w-full rounded-2xl border border-border/80 bg-background/85 px-4 py-3 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                    />
                  </div>

                  <div>
                    <label
                      htmlFor="webui-port"
                      className="block text-sm font-medium text-foreground mb-1.5"
                    >
                      {t('initWebUIPort')}
                    </label>
                    <input
                      id="webui-port"
                      type="number"
                      min="0"
                      value={webuiPort}
                      onChange={(e) => setWebuiPort(e.target.value)}
                      className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                    />
                  </div>

                  <div className="md:col-span-2">
                    <label
                      htmlFor="public-base-url"
                      className="block text-sm font-medium text-foreground mb-1.5"
                    >
                      {t('initPublicBaseURL')}
                    </label>
                    <input
                      id="public-base-url"
                      type="url"
                      value={publicBaseURL}
                      onChange={(e) => setPublicBaseURL(e.target.value)}
                      className="w-full rounded-2xl border border-border/80 bg-background/85 px-4 py-3 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                      placeholder="https://bot.example.com"
                    />
                  </div>
                </div>

                <label className="flex items-start gap-3 rounded-2xl border border-border/80 bg-card/80 px-4 py-4 backdrop-blur-xl">
                  <input
                    type="checkbox"
                    checked={webuiEnabled}
                    onChange={(e) => setWebuiEnabled(e.target.checked)}
                    className="mt-1 h-4 w-4 rounded border-border"
                  />
                  <span>
                    <span className="block text-sm font-medium text-foreground">{t('initWebUIEnabled')}</span>
                    <span className="block text-xs text-muted-foreground">{t('initWebUIEnabledDesc')}</span>
                  </span>
                </label>

                <div className="rounded-2xl border border-amber-300/80 bg-amber-50/90 px-4 py-3 text-xs leading-5 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-100">
                  {t('initRestartNotice')}
                </div>

                <div className="rounded-2xl border border-border/70 bg-muted/55 px-4 py-3 text-xs leading-5 text-muted-foreground">
                  {t('initStorageReadonly')}
                </div>
              </div>
            </div>

            <Button
              type="submit"
              disabled={loading || !username.trim() || !password.trim()}
              className="h-11 w-full rounded-2xl"
            >
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {loading ? t('initSubmittingButton') : t('initialize')}
            </Button>
          </form>
        </div>
      </div>
    </AuthGradientShell>
  );
}
