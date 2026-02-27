import { t } from '@/lib/i18n';
import { useStatus } from '@/hooks/useConfig';
import Header from '@/components/layout/Header';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Button } from '@/components/ui/button';
import { RefreshCw } from 'lucide-react';

export default function SystemPage() {
  const { data: status, isLoading, refetch, isFetching } = useStatus();

  return (
    <div className="flex flex-col h-full">
      <Header title={t('tabStatus')} />
      <div className="flex items-center gap-2 px-6 pb-4">
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetch()}
          disabled={isFetching}
        >
          <RefreshCw className={`h-4 w-4 mr-1 ${isFetching ? 'animate-spin' : ''}`} />
          {t('refresh')}
        </Button>
      </div>

      <ScrollArea className="flex-1 px-6 pb-6">
        {isLoading ? (
          <div className="text-muted-foreground py-8 text-center animate-pulse">Loading\u2026</div>
        ) : (
          <pre className="rounded-lg border border-border bg-card p-4 text-sm font-mono overflow-auto whitespace-pre-wrap break-words">
            {JSON.stringify(status, null, 2)}
          </pre>
        )}
      </ScrollArea>
    </div>
  );
}
