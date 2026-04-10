import { useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { usePolicyPresets, useEvaluatePolicy } from '@/hooks/usePolicy';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { t } from '@/lib/i18n';

export default function PolicyPage() {
  const { data: presets = [] } = usePolicyPresets();
  const evaluate = useEvaluatePolicy();
  const [selected, setSelected] = useState('permissive');
  const [mode, setMode] = useState<'tool' | 'filesystem' | 'network'>('tool');
  const [toolName, setToolName] = useState('exec');
  const [path, setPath] = useState('/workspace/project/README.md');
  const [write, setWrite] = useState(false);
  const [host, setHost] = useState('api.openai.com');
  const [method, setMethod] = useState('POST');
  const [urlPath, setUrlPath] = useState('/v1/chat/completions');

  const policy = useMemo(
    () => presets.find((item) => item.name === selected) ?? presets[0] ?? null,
    [presets, selected],
  );

  return (
    <div className="space-y-6">
      <Header title={t('tabPolicy')} description={t('policyDescription')} />

      <Card>
        <CardHeader>
          <CardTitle>{t('policyPresetsTitle')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-2">
            {presets.map((preset) => (
              <Button
                key={preset.name}
                variant={selected === preset.name ? 'default' : 'outline'}
                onClick={() => setSelected(preset.name)}
              >
                {preset.name}
              </Button>
            ))}
          </div>
          {policy ? (
            <pre className="rounded-2xl border border-border/70 bg-muted/40 p-4 text-xs whitespace-pre-wrap">
              {JSON.stringify(policy, null, 2)}
            </pre>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('policyEvaluatorTitle')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap gap-2">
            <Button variant={mode === 'tool' ? 'default' : 'outline'} onClick={() => setMode('tool')}>{t('policyModeTool')}</Button>
            <Button variant={mode === 'filesystem' ? 'default' : 'outline'} onClick={() => setMode('filesystem')}>{t('policyModeFilesystem')}</Button>
            <Button variant={mode === 'network' ? 'default' : 'outline'} onClick={() => setMode('network')}>{t('policyModeNetwork')}</Button>
          </div>

          {mode === 'tool' ? (
            <div className="space-y-2">
              <Label>{t('policyToolName')}</Label>
              <Input value={toolName} onChange={(e) => setToolName(e.target.value)} />
            </div>
          ) : null}

          {mode === 'filesystem' ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>{t('path')}</Label>
                <Input value={path} onChange={(e) => setPath(e.target.value)} />
              </div>
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={write} onChange={(e) => setWrite(e.target.checked)} />
                {t('policyWriteAccess')}
              </label>
            </div>
          ) : null}

          {mode === 'network' ? (
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label>{t('host')}</Label>
                <Input value={host} onChange={(e) => setHost(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>{t('method')}</Label>
                <Input value={method} onChange={(e) => setMethod(e.target.value)} />
              </div>
              <div className="space-y-2 md:col-span-2">
                <Label>{t('path')}</Label>
                <Input value={urlPath} onChange={(e) => setUrlPath(e.target.value)} />
              </div>
            </div>
          ) : null}

          <Button
            disabled={!policy || evaluate.isPending}
            onClick={() =>
              policy &&
              evaluate.mutate({
                policy,
                input:
                  mode === 'tool'
                    ? { tool_name: toolName }
                    : mode === 'filesystem'
                      ? { path, write }
                      : { host, method, url_path: urlPath },
              })
            }
          >
            {t('policyEvaluate')}
          </Button>
          {evaluate.data ? (
            <pre className="rounded-2xl border border-border/70 bg-muted/40 p-4 text-xs whitespace-pre-wrap">
              {JSON.stringify(evaluate.data, null, 2)}
            </pre>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
