import { cn } from '@/lib/utils';
import { t } from '@/lib/i18n';
import type { ChatMessage } from '@/hooks/useChat';

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  });
}

interface MessageBubbleProps {
  message: ChatMessage;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  if (message.role === 'user') {
    return (
      <div className="flex justify-end">
        <div className="max-w-[82%] space-y-2">
          <div className="rounded-[1.4rem] rounded-br-md bg-[hsl(var(--gray-900))] px-4 py-3 text-sm leading-6 text-white shadow-[0_16px_40px_-24px_rgba(20,15,10,0.75)] whitespace-pre-wrap break-words">
            {message.content}
          </div>
          <div className="eyebrow-label mono-data text-right text-muted-foreground/80">
            {formatTime(message.timestamp)}
          </div>
        </div>
      </div>
    );
  }

  if (message.role === 'assistant') {
    return (
      <div className="flex justify-start">
        <div className="max-w-[88%] space-y-2">
          <div className="rounded-[1.4rem] rounded-bl-md border border-[hsl(var(--brand-200))] bg-white/90 px-4 py-3 text-sm leading-6 text-foreground shadow-[0_18px_42px_-30px_rgba(120,55,75,0.35)] backdrop-blur whitespace-pre-wrap break-words">
            {message.content}
          </div>
          <div className="eyebrow-label mono-data text-muted-foreground/80">
            {formatTime(message.timestamp)}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex justify-center">
      <div
        className={cn(
          'max-w-[90%] rounded-full px-3 py-1.5 text-xs',
          message.role === 'error'
            ? 'bg-destructive/10 text-destructive'
            : 'bg-[hsl(var(--gray-100))] text-muted-foreground',
        )}
      >
        {message.content}
      </div>
    </div>
  );
}
