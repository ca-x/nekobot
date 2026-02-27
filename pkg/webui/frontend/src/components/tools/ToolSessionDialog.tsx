import { useState, useEffect, useCallback } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { t } from '@/lib/i18n';
import {
  useCreateToolSession,
  useUpdateToolSession,
  type ToolSession,
  type CreateToolSessionPayload,
} from '@/hooks/useToolSessions';

const TOOL_PRESETS = [
  { value: 'codex', label: 'Codex' },
  { value: 'opencode', label: 'OpenCode' },
  { value: 'claude', label: 'Claude Code' },
  { value: 'aider', label: 'Aider' },
  { value: '__custom__', label: '' }, // label set dynamically with t()
];

const PROXY_MODES = [
  { value: 'inherit', labelKey: 'proxyModeInherit' },
  { value: 'clear', labelKey: 'proxyModeClear' },
  { value: 'custom', labelKey: 'proxyModeCustom' },
];

const ACCESS_MODES = [
  { value: 'none', label: 'None' },
  { value: 'one_time', label: 'One-time password' },
  { value: 'permanent', label: 'Permanent password' },
];

const DRAFT_KEY = 'nekobot_tool_session_draft';

function loadDraft(): Partial<CreateToolSessionPayload> {
  try {
    const raw = localStorage.getItem(DRAFT_KEY) || '{}';
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function saveDraft(draft: Partial<CreateToolSessionPayload>) {
  try {
    localStorage.setItem(DRAFT_KEY, JSON.stringify(draft));
  } catch {
    /* ignore */
  }
}

interface ToolSessionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editSession?: ToolSession | null;
  onCreated?: (session: ToolSession, accessUrl?: string, accessPassword?: string) => void;
}

export default function ToolSessionDialog({
  open,
  onOpenChange,
  editSession,
  onCreated,
}: ToolSessionDialogProps) {
  const isEdit = !!editSession;

  const [tool, setTool] = useState('codex');
  const [customTool, setCustomTool] = useState('');
  const [title, setTitle] = useState('');
  const [commandArgs, setCommandArgs] = useState('');
  const [workdir, setWorkdir] = useState('');
  const [proxyMode, setProxyMode] = useState('inherit');
  const [proxyUrl, setProxyUrl] = useState('');
  const [accessMode, setAccessMode] = useState('none');
  const [accessPassword, setAccessPassword] = useState('');
  const [publicBaseUrl, setPublicBaseUrl] = useState('');

  const createMutation = useCreateToolSession();
  const updateMutation = useUpdateToolSession();

  const resetForm = useCallback(() => {
    if (editSession) {
      const meta =
        editSession.metadata && typeof editSession.metadata === 'object'
          ? editSession.metadata
          : {};
      const toolVal = String(editSession.tool || '').trim();
      const isPreset = TOOL_PRESETS.some(
        (p) => p.value === toolVal && p.value !== '__custom__',
      );
      if (isPreset) {
        setTool(toolVal);
        setCustomTool('');
      } else {
        setTool('__custom__');
        setCustomTool(toolVal);
      }
      setTitle(String(editSession.title || '').trim());
      const userArgs = String((meta as Record<string, unknown>).user_args || '').trim();
      if (userArgs) {
        setCommandArgs(userArgs);
      } else {
        const cmd = String(editSession.command || '').trim();
        setCommandArgs(inferCommandArgs(toolVal, cmd));
      }
      setWorkdir(String(editSession.workdir || '').trim());
      setProxyMode(
        String((meta as Record<string, unknown>).proxy_mode || 'inherit')
          .trim()
          .toLowerCase() || 'inherit',
      );
      setProxyUrl(
        String((meta as Record<string, unknown>).proxy_url || '').trim(),
      );
      setAccessMode(
        String(editSession.access_mode || 'none').trim().toLowerCase() || 'none',
      );
      setAccessPassword('');
      setPublicBaseUrl('');
    } else {
      const draft = loadDraft();
      const draftTool = String(draft.tool || 'codex').trim();
      const isPreset = TOOL_PRESETS.some(
        (p) => p.value === draftTool && p.value !== '__custom__',
      );
      if (isPreset) {
        setTool(draftTool);
        setCustomTool('');
      } else {
        setTool('__custom__');
        setCustomTool(draftTool);
      }
      setTitle('');
      setCommandArgs(String(draft.command_args || '').trim());
      setWorkdir(String(draft.workdir || '').trim());
      setProxyMode(
        String(draft.proxy_mode || 'inherit').trim().toLowerCase() || 'inherit',
      );
      setProxyUrl(String(draft.proxy_url || '').trim());
      setAccessMode(
        String(draft.access_mode || 'none').trim().toLowerCase() || 'none',
      );
      setAccessPassword('');
      setPublicBaseUrl(String(draft.public_base_url || '').trim());
    }
  }, [editSession]);

  useEffect(() => {
    if (open) resetForm();
  }, [open, resetForm]);

  function inferCommandArgs(toolValue: string, commandValue: string): string {
    const toolStr = toolValue.trim();
    const cmd = commandValue.trim();
    if (!cmd) return '';
    if (!toolStr) return cmd;
    if (cmd === toolStr) return '';
    const prefix = toolStr + ' ';
    if (cmd.startsWith(prefix)) return cmd.slice(prefix.length).trim();
    return cmd;
  }

  function getEffectiveTool(): string {
    return tool === '__custom__' ? customTool.trim() : tool;
  }

  async function handleSubmit() {
    const effectiveTool = getEffectiveTool();
    if (!effectiveTool) return;

    const effectiveProxyMode =
      proxyMode === 'clear' || proxyMode === 'custom' ? proxyMode : 'inherit';
    const effectiveProxyUrl = effectiveProxyMode === 'custom' ? proxyUrl.trim() : '';

    if (effectiveProxyMode === 'custom' && !effectiveProxyUrl) {
      return;
    }

    const payload: CreateToolSessionPayload = {
      tool: effectiveTool,
      title: title.trim(),
      command_args: commandArgs.trim(),
      workdir: workdir.trim(),
      proxy_mode: effectiveProxyMode,
      proxy_url: effectiveProxyUrl,
      access_mode: accessMode,
      access_password: accessPassword.trim(),
      public_base_url: publicBaseUrl.trim(),
    };

    /* Save draft for next time */
    saveDraft({
      tool: effectiveTool,
      command_args: payload.command_args,
      workdir: payload.workdir,
      proxy_mode: payload.proxy_mode,
      proxy_url: payload.proxy_url,
      access_mode: payload.access_mode,
      public_base_url: payload.public_base_url,
    });

    try {
      if (isEdit && editSession) {
        const result = await updateMutation.mutateAsync({
          id: editSession.id,
          payload,
        });
        onOpenChange(false);
        if (result?.session && onCreated) {
          onCreated(
            result.session,
            result.access_url,
            result.access_password,
          );
        }
      } else {
        const result = await createMutation.mutateAsync(payload);
        onOpenChange(false);
        if (result?.session && onCreated) {
          onCreated(
            result.session,
            result.access_url,
            result.access_password,
          );
        }
      }
    } catch {
      /* error handled by mutation */
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t('editToolSessionTitle') : t('newToolSessionTitle')}
          </DialogTitle>
          <DialogDescription className="sr-only">
            {isEdit ? t('editToolSessionTitle') : t('newToolSessionTitle')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Tool selection */}
          <div className="space-y-2">
            <Label>{t('toolName')}</Label>
            <Select value={tool} onValueChange={setTool}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {TOOL_PRESETS.map((p) => (
                  <SelectItem key={p.value} value={p.value}>
                    {p.value === '__custom__' ? t('customTool') : p.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {tool === '__custom__' && (
              <Input
                placeholder={t('customTool')}
                value={customTool}
                onChange={(e) => setCustomTool(e.target.value)}
              />
            )}
          </div>

          {/* Session title */}
          <div className="space-y-2">
            <Label>{t('sessionTitle')}</Label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t('sessionTitle')}
            />
          </div>

          {/* Command args */}
          <div className="space-y-2">
            <Label>{t('launchCommand')}</Label>
            <Input
              value={commandArgs}
              onChange={(e) => setCommandArgs(e.target.value)}
              placeholder={t('launchCommand')}
            />
            <p className="text-xs text-muted-foreground">
              {t('launchCommandHelp')}
            </p>
          </div>

          {/* Working directory */}
          <div className="space-y-2">
            <Label>{t('workingDirectory')}</Label>
            <Input
              value={workdir}
              onChange={(e) => setWorkdir(e.target.value)}
              placeholder={t('workingDirectory')}
            />
          </div>

          {/* Proxy mode */}
          <div className="space-y-2">
            <Label>{t('proxyMode')}</Label>
            <Select value={proxyMode} onValueChange={setProxyMode}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROXY_MODES.map((m) => (
                  <SelectItem key={m.value} value={m.value}>
                    {t(m.labelKey)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {proxyMode === 'custom' && (
              <Input
                placeholder="http://proxy:port\u2026"
                value={proxyUrl}
                onChange={(e) => setProxyUrl(e.target.value)}
              />
            )}
          </div>

          {/* Access mode */}
          <div className="space-y-2">
            <Label>{t('accessMode')}</Label>
            <Select value={accessMode} onValueChange={setAccessMode}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ACCESS_MODES.map((m) => (
                  <SelectItem key={m.value} value={m.value}>
                    {m.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Access password (only shown for permanent) */}
          {accessMode === 'permanent' && (
            <div className="space-y-2">
              <Label>{t('accessPassword')}</Label>
              <Input
                value={accessPassword}
                onChange={(e) => setAccessPassword(e.target.value)}
                placeholder={t('accessPassword')}
              />
            </div>
          )}

          {/* Public base URL */}
          {accessMode !== 'none' && (
            <div className="space-y-2">
              <Label>{t('publicBaseUrl')}</Label>
              <Input
                value={publicBaseUrl}
                onChange={(e) => setPublicBaseUrl(e.target.value)}
                placeholder="https://example.com\u2026"
              />
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={isPending || !getEffectiveTool()}>
            {isEdit ? t('save') : t('createSession')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
