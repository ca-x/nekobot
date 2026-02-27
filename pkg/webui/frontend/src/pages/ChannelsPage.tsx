import { useState } from 'react';
import Header from '@/components/layout/Header';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { useChannels, useTestChannel } from '@/hooks/useChannels';
import { ChannelForm } from '@/components/config/ChannelForm';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

/** Emoji icons for known channel types; unknown channels get their first letter. */
const channelEmoji: Record<string, string> = {
  telegram: 'TG',
  discord: 'DC',
  slack: 'SK',
  whatsapp: 'WA',
  feishu: 'FS',
  dingtalk: 'DT',
  qq: 'QQ',
  wework: 'WW',
  serverchan: 'SC',
  googlechat: 'GC',
  maixcam: 'MX',
  teams: 'TM',
  infoflow: 'IF',
  email: 'EM',
};

/** Background color per channel (for the badge). */
const channelColors: Record<string, string> = {
  telegram: 'bg-sky-500',
  discord: 'bg-indigo-500',
  slack: 'bg-emerald-600',
  whatsapp: 'bg-green-500',
  feishu: 'bg-blue-500',
  dingtalk: 'bg-blue-600',
  qq: 'bg-cyan-500',
  wework: 'bg-teal-600',
  serverchan: 'bg-amber-500',
  googlechat: 'bg-green-600',
  maixcam: 'bg-orange-500',
  teams: 'bg-violet-600',
  infoflow: 'bg-rose-500',
  email: 'bg-gray-500',
};

export default function ChannelsPage() {
  const { data: channels, isLoading } = useChannels();
  const testChannel = useTestChannel();

  const [activeTab, setActiveTab] = useState<string>('all');
  const [editingChannel, setEditingChannel] = useState<string | null>(null);

  // Derive channel list from the map.
  const allChannels = channels
    ? Object.entries(channels).sort(([a], [b]) => a.localeCompare(b))
    : [];

  const enabledCount = allChannels.filter(([, cfg]) => cfg.enabled).length;

  const filteredChannels =
    activeTab === 'enabled'
      ? allChannels.filter(([, cfg]) => cfg.enabled)
      : allChannels;

  const handleTest = (name: string, e: React.MouseEvent) => {
    e.stopPropagation();
    testChannel.mutate(name);
  };

  return (
    <div>
      <Header title={t('tabChannels')} description={t('channelsPageDescription')} />

      {/* Tab filter */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="mb-6">
        <TabsList>
          <TabsTrigger value="all">
            {t('channelsTabAll')} ({allChannels.length})
          </TabsTrigger>
          <TabsTrigger value="enabled">
            {t('channelsTabEnabled')} ({enabledCount})
          </TabsTrigger>
        </TabsList>
      </Tabs>

      {/* Loading state */}
      {isLoading && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-36 rounded-xl" />
          ))}
        </div>
      )}

      {/* Channel card grid */}
      {!isLoading && filteredChannels.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {filteredChannels.map(([name, config]) => {
            const enabled = config.enabled ?? false;
            const badge = channelEmoji[name] ?? name.slice(0, 2).toUpperCase();
            const badgeColor = channelColors[name] ?? 'bg-gray-400';

            return (
              <Card
                key={name}
                className={cn(
                  'cursor-pointer transition-[shadow,border-color] hover:shadow-md hover:border-primary/30',
                  enabled && 'border-primary/20',
                )}
                onClick={() => setEditingChannel(name)}
              >
                <CardContent className="p-5">
                  <div className="flex items-start justify-between mb-3">
                    {/* Channel badge */}
                    <div
                      className={cn(
                        'flex h-10 w-10 items-center justify-center rounded-lg text-white text-xs font-bold',
                        badgeColor,
                      )}
                    >
                      {badge}
                    </div>

                    {/* Enabled / Disabled indicator */}
                    <span
                      className={cn(
                        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium',
                        enabled
                          ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                          : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
                      )}
                    >
                      <span
                        className={cn(
                          'h-1.5 w-1.5 rounded-full',
                          enabled ? 'bg-emerald-500' : 'bg-gray-400',
                        )}
                      />
                      {enabled ? t('on') : t('off')}
                    </span>
                  </div>

                  {/* Channel name */}
                  <h3 className="text-sm font-semibold capitalize mb-3 text-foreground">
                    {name}
                  </h3>

                  {/* Actions */}
                  <div className="flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingChannel(name);
                      }}
                    >
                      {t('edit')}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 text-xs"
                      disabled={!enabled || testChannel.isPending}
                      onClick={(e) => handleTest(name, e)}
                    >
                      {testChannel.isPending && testChannel.variables === name
                        ? t('testing')
                        : t('test')}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && filteredChannels.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="h-14 w-14 flex items-center justify-center rounded-xl bg-muted mb-4">
            <span className="text-2xl text-muted-foreground">#</span>
          </div>
          <h3 className="text-sm font-semibold text-foreground mb-1.5">
            {t('channelsEmptyTitle')}
          </h3>
          <p className="text-sm text-muted-foreground max-w-sm">
            {t('channelsEmptyDescription')}
          </p>
        </div>
      )}

      {/* Channel edit dialog */}
      <ChannelForm
        open={editingChannel !== null}
        channelName={editingChannel}
        channelConfig={editingChannel && channels ? channels[editingChannel] ?? null : null}
        onClose={() => setEditingChannel(null)}
      />
    </div>
  );
}
