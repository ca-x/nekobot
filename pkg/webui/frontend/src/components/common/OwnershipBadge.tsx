import { ShieldCheck, Users, UserRound } from 'lucide-react';

import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

export type ResourceVisibility = 'private' | 'shared' | 'system';

export interface OwnedResourceFields {
  tenant_id?: string;
  owner_user_id?: string;
  visibility?: ResourceVisibility | string;
}

export function normalizeVisibility(value?: string): ResourceVisibility {
  if (value === 'private' || value === 'system') {
    return value;
  }
  return 'shared';
}

export function visibilityLabel(value?: string): string {
  const visibility = normalizeVisibility(value);
  if (visibility === 'private') return t('visibilityPrivate');
  if (visibility === 'system') return t('visibilitySystem');
  return t('visibilityShared');
}

export function OwnershipBadge({
  resource,
  className,
  showOwner = false,
}: {
  resource: OwnedResourceFields;
  className?: string;
  showOwner?: boolean;
}) {
  const visibility = normalizeVisibility(resource.visibility);
  const Icon = visibility === 'private' ? UserRound : visibility === 'system' ? ShieldCheck : Users;
  return (
    <div className={cn('flex flex-wrap items-center gap-1.5', className)}>
      <span
        className={cn(
          'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium',
          visibility === 'private' && 'bg-sky-500/12 text-sky-700 dark:text-sky-300',
          visibility === 'shared' && 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-300',
          visibility === 'system' && 'bg-amber-500/14 text-amber-800 dark:text-amber-300',
        )}
      >
        <Icon className="h-3.5 w-3.5" />
        {visibilityLabel(visibility)}
      </span>
      {showOwner && resource.owner_user_id ? (
        <span className="inline-flex max-w-full items-center rounded-full bg-muted px-2.5 py-1 text-[11px] text-muted-foreground">
          <span className="truncate">{resource.owner_user_id}</span>
        </span>
      ) : null}
    </div>
  );
}
