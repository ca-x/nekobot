import { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useUpdateChannel, type ChannelConfig } from '@/hooks/useChannels';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';

interface ChannelFormProps {
  open: boolean;
  channelName: string | null;
  channelConfig: ChannelConfig | null;
  onClose: () => void;
}

/**
 * Determine the field rendering type from a key name and its current value.
 */
function fieldType(
  key: string,
  value: unknown,
): 'boolean' | 'password' | 'number' | 'string' | 'json' {
  if (typeof value === 'boolean') return 'boolean';
  if (typeof value === 'number') return 'number';
  const lower = key.toLowerCase();
  if (
    lower.includes('password') ||
    lower.includes('secret') ||
    lower.includes('token') ||
    lower.includes('aes_key')
  ) {
    return 'password';
  }
  if (typeof value === 'string') return 'string';
  return 'json';
}

/**
 * Pretty-print a snake_case or camelCase key as a human label.
 */
function prettyLabel(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export function ChannelForm({ open, channelName, channelConfig, onClose }: ChannelFormProps) {
  const updateChannel = useUpdateChannel();

  const [formData, setFormData] = useState<Record<string, unknown>>({});
  const [jsonDrafts, setJsonDrafts] = useState<Record<string, string>>({});

  // Sync form state when the dialog opens or channel changes.
  useEffect(() => {
    if (channelConfig) {
      setFormData({ ...channelConfig });
      const drafts: Record<string, string> = {};
      for (const [key, value] of Object.entries(channelConfig)) {
        if (fieldType(key, value) === 'json') {
          drafts[key] = JSON.stringify(value ?? null, null, 2);
        }
      }
      setJsonDrafts(drafts);
    } else {
      setFormData({});
      setJsonDrafts({});
    }
  }, [channelConfig, channelName]);

  const updateField = (name: string, value: unknown) => {
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!channelName) return;

    const payload: Record<string, unknown> = { ...formData };

    // Merge JSON drafts back into the payload.
    for (const [key, raw] of Object.entries(jsonDrafts)) {
      try {
        payload[key] = raw.trim() ? JSON.parse(raw) : null;
      } catch {
        toast.error(t('invalidJson', key));
        return;
      }
    }

    updateChannel.mutate(
      { name: channelName, data: payload },
      { onSuccess: () => onClose() },
    );
  };

  // Build ordered field list: "enabled" first, then alphabetical.
  const fieldKeys = Object.keys(formData).sort((a, b) => {
    if (a === 'enabled') return -1;
    if (b === 'enabled') return 1;
    return a.localeCompare(b);
  });

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-[560px] max-h-[85vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="capitalize">
            {t('editChannelTitle', channelName ?? '')}
          </DialogTitle>
          <DialogDescription>
            {t('channelFormDescription')}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
          <ScrollArea className="flex-1 max-h-[55vh] pr-3">
            <div className="space-y-5 py-2">
              {fieldKeys.map((key) => {
                const value = formData[key];
                const ft = fieldType(key, channelConfig?.[key] ?? value);

                return (
                  <div key={key} className="space-y-2">
                    <Label htmlFor={key} className="text-sm font-medium">
                      {prettyLabel(key)}
                    </Label>

                    {ft === 'boolean' && (
                      <div className="flex items-center justify-between rounded-lg border bg-muted/40 px-3 py-2.5">
                        <span className="text-sm text-muted-foreground">
                          {(value as boolean) ? t('on') : t('off')}
                        </span>
                        <Switch
                          id={key}
                          checked={(value as boolean) || false}
                          onCheckedChange={(checked) => updateField(key, checked)}
                        />
                      </div>
                    )}

                    {ft === 'password' && (
                      <Input
                        id={key}
                        type="password"
                        value={(value as string) ?? ''}
                        onChange={(e) => updateField(key, e.target.value)}
                        placeholder={t('channelFieldPasswordHint')}
                        autoComplete="off"
                      />
                    )}

                    {ft === 'string' && (
                      <Input
                        id={key}
                        type="text"
                        value={(value as string) ?? ''}
                        onChange={(e) => updateField(key, e.target.value)}
                      />
                    )}

                    {ft === 'number' && (
                      <Input
                        id={key}
                        type="number"
                        value={(value as number) ?? 0}
                        onChange={(e) =>
                          updateField(key, e.target.value === '' ? 0 : Number(e.target.value))
                        }
                      />
                    )}

                    {ft === 'json' && (
                      <textarea
                        id={key}
                        value={jsonDrafts[key] ?? ''}
                        onChange={(e) =>
                          setJsonDrafts((prev) => ({ ...prev, [key]: e.target.value }))
                        }
                        className="min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 text-xs font-mono ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                      />
                    )}
                  </div>
                );
              })}

              {fieldKeys.length === 0 && (
                <p className="text-sm text-muted-foreground py-4 text-center">
                  {t('channelNoFields')}
                </p>
              )}
            </div>
          </ScrollArea>

          <DialogFooter className="pt-4 flex-shrink-0">
            <Button type="button" variant="outline" onClick={onClose}>
              {t('cancel')}
            </Button>
            <Button type="submit" disabled={updateChannel.isPending}>
              {updateChannel.isPending ? t('saving') : t('save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
