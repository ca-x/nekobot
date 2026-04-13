import { useEffect, useState } from 'react';
import Header from '@/components/layout/Header';
import { useConfig } from '@/hooks/useConfig';
import { useSaveWebhookConfig, useTestWebhook } from '@/hooks/useWebhook';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { t } from '@/lib/i18n';

export default function WebhooksPage() {
  const { data: config } = useConfig();
  const saveWebhook = useSaveWebhookConfig();
  const testWebhook = useTestWebhook();

  const current = (config?.webhook as { enabled?: boolean; path?: string } | undefined) ?? {};
  const [enabled, setEnabled] = useState(Boolean(current.enabled));
  const [path, setPath] = useState(current.path ?? '/api/webhooks/agent');
  const [message, setMessage] = useState('hello from webhook test');
  const canSendTest = message.trim() !== '';

  useEffect(() => {
    setEnabled(Boolean(current.enabled));
    setPath(current.path ?? '/api/webhooks/agent');
  }, [current.enabled, current.path]);

  return (
    <div className="space-y-6">
      <Header title={t('tabWebhooks')} description={t('webhooksDescription')} />

      <Card>
        <CardHeader>
          <CardTitle>{t('webhooksTriggerTitle')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-2xl border border-border/70 p-4">
            <div>
              <Label className="text-sm font-semibold">{t('enabled')}</Label>
              <div className="text-xs text-muted-foreground">{t('webhooksEnabledDescription')}</div>
            </div>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>

          <div className="space-y-2">
            <Label>{t('path')}</Label>
            <Input value={path} onChange={(e) => setPath(e.target.value)} />
            <p className="text-xs text-muted-foreground">
              {t('webhooksPathHint')}
            </p>
          </div>

          <div className="flex gap-2">
            <Button
              onClick={() => saveWebhook.mutate({ enabled, path })}
              disabled={saveWebhook.isPending}
            >
              {t('save')}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('webhooksTestTitle')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>{t('message')}</Label>
            <Input value={message} onChange={(e) => setMessage(e.target.value)} />
          </div>
          <Button
            onClick={() => {
              const trimmed = message.trim();
              if (!trimmed) return;
              testWebhook.mutate(trimmed);
            }}
            disabled={testWebhook.isPending || !canSendTest}
          >
            {t('webhooksSendTest')}
          </Button>
          <p className="text-xs text-muted-foreground">
            {t('webhooksTestHint')}
          </p>
          {testWebhook.data ? (
            <pre className="rounded-2xl border border-border/70 bg-muted/40 p-4 text-xs whitespace-pre-wrap">
              {JSON.stringify(testWebhook.data, null, 2)}
            </pre>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
