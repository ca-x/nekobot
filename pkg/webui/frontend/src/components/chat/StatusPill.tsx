import { cn } from '@/lib/utils';
import { t } from '@/lib/i18n';

interface StatusPillProps {
  status: string;
  isAwaitingReply: boolean;
}

export function StatusPill({ status, isAwaitingReply }: StatusPillProps) {
  const colorClass =
    status === 'connected'
      ? 'bg-emerald-500'
      : status === 'connecting'
        ? 'bg-amber-500 animate-pulse'
        : 'bg-rose-500';

  const label =
    status === 'connected'
      ? t('wsConnected')
      : status === 'connecting'
        ? t('chatConnecting')
        : t('wsDisconnected');

  return (
    <div className="flex max-w-full flex-row flex-wrap items-center justify-start gap-2 sm:justify-end">
      <div className="inline-flex max-w-full items-center gap-2 rounded-full border border-border/70 bg-card/92 px-3.5 py-2 text-xs text-muted-foreground shadow-sm backdrop-blur">
        <span className={cn('h-2.5 w-2.5 shrink-0 rounded-full', colorClass)} />
        <span className="min-w-0 whitespace-nowrap font-medium">{label}</span>
      </div>
      {isAwaitingReply && (
        <span className="inline-flex h-7 items-center rounded-full bg-accent px-2.5 text-[11px] font-medium text-accent-foreground whitespace-nowrap">
          {t('chatWaitingReply')}
        </span>
      )}
    </div>
  );
}
