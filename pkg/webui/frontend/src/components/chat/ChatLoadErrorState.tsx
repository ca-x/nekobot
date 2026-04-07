import { RefreshCw, AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { t } from '@/lib/i18n';

interface ChatLoadErrorStateProps {
  title: string;
  description: string;
  message: string;
  onRetry: () => void;
  retrying: boolean;
}

export function ChatLoadErrorState({
  title,
  description,
  message,
  onRetry,
  retrying,
}: ChatLoadErrorStateProps) {
  return (
    <div className="rounded-[1.5rem] border border-rose-200/80 bg-rose-50/70 p-4 shadow-[0_20px_50px_-38px_rgba(160,60,70,0.4)]">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-rose-100 text-rose-700">
          <AlertCircle className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold text-rose-950">{title}</div>
          <p className="mt-1 text-xs leading-5 text-rose-900/80">{description}</p>
          <div className="mt-3 rounded-2xl border border-rose-200/80 bg-white/90 px-3 py-2 text-xs text-rose-950">
            {message}
          </div>
        </div>
      </div>
      <div className="mt-3 flex justify-end">
        <Button type="button" variant="outline" className="rounded-full" onClick={onRetry} disabled={retrying}>
          <RefreshCw className={`mr-2 h-4 w-4 ${retrying ? 'animate-spin' : ''}`} />
          {t('refresh')}
        </Button>
      </div>
    </div>
  );
}
