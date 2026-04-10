import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { api, setToken } from '@/api/client';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';
import { PasswordInput } from '@/components/ui/password-input';
import { Button } from '@/components/ui/button';

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
      } else {
        setSubmitStatus('idle');
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
      <div className="flex h-screen items-center justify-center bg-background p-4">
        <div className="text-sm text-muted-foreground">{t('checkingInit')}</div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-4xl">
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
          <p className="text-center text-sm text-muted-foreground mb-1">
            {t('initTitle')}
          </p>
          <p className="text-center text-xs text-muted-foreground mb-6">
            {t('firstRunHint')}
          </p>

          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="grid gap-6 lg:grid-cols-2">
              <div className="space-y-4 rounded-2xl border border-border/70 bg-background/70 p-5">
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
                    className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
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

                <div className="rounded-xl border border-border/70 bg-muted/40 px-3 py-3 text-xs text-muted-foreground">
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

              <div className="space-y-4 rounded-2xl border border-border/70 bg-background/70 p-5">
                <div>
                  <div className="text-sm font-semibold text-foreground">{t('initBootstrapTitle')}</div>
                  <p className="mt-1 text-xs text-muted-foreground">{t('initBootstrapDesc')}</p>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="md:col-span-2">
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      {t('initConfigPath')}
                    </label>
                    <div className="w-full rounded-xl border border-border bg-muted px-3 py-2 text-sm text-muted-foreground break-all">
                      {configPath}
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      {t('initDBDir')}
                    </label>
                    <div className="w-full rounded-xl border border-border bg-muted px-3 py-2 text-sm text-muted-foreground break-all">
                      {dbDir}
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      {t('initWorkspace')}
                    </label>
                    <div className="w-full rounded-xl border border-border bg-muted px-3 py-2 text-sm text-muted-foreground break-all">
                      {workspace}
                    </div>
                  </div>

                  <div className="md:col-span-2 rounded-xl border border-border/70 bg-muted/40 px-3 py-3 text-xs text-muted-foreground">
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

                  <div className="md:col-span-2 rounded-xl border border-border/70 bg-muted/40 px-3 py-3 text-xs text-muted-foreground">
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
                      className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
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
                      className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
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
                      className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
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
                      className="w-full rounded-xl border border-border bg-input px-3 py-2 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 transition-shadow"
                      placeholder="https://bot.example.com"
                    />
                  </div>
                </div>

                <label className="flex items-start gap-3 rounded-xl border border-border/80 bg-card px-3 py-3">
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

                <div className="rounded-xl border border-amber-300/80 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-900">
                  {t('initRestartNotice')}
                </div>

                <div className="rounded-xl border border-border/70 bg-muted/60 px-4 py-3 text-xs leading-5 text-muted-foreground">
                  {t('initStorageReadonly')}
                </div>
              </div>
            </div>

            <Button
              type="submit"
              disabled={loading || !username.trim() || !password.trim()}
              className="w-full"
            >
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {loading ? t('initSubmittingButton') : t('initialize')}
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}
