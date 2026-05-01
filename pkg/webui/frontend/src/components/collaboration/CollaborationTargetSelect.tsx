import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { CollaborationTargetOption } from '@/hooks/useCollaborationTargets';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

interface CollaborationTargetSelectProps {
  value: string;
  options: CollaborationTargetOption[];
  onValueChange: (value: string) => void;
  disabled?: boolean;
  placeholder?: string;
  className?: string;
}

export function CollaborationTargetSelect({
  value,
  options,
  onValueChange,
  disabled,
  placeholder,
  className,
}: CollaborationTargetSelectProps) {
  return (
    <Select value={value || undefined} onValueChange={onValueChange} disabled={disabled || options.length === 0}>
      <SelectTrigger className={cn('h-11 w-full', className)}>
        <SelectValue placeholder={placeholder || t('collaborationTargetSelectPlaceholder')} />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.target} value={option.target} disabled={option.disabled}>
            <span className="flex min-w-0 flex-col gap-0.5">
              <span className="truncate text-sm">{option.label}</span>
              <span className="truncate text-[11px] text-muted-foreground">
                {t(`collaborationTargetKind_${option.kind}`)} · {option.target}
              </span>
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
