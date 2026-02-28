import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useDisableMarketplaceSkill,
  useEnableMarketplaceSkill,
  useMarketplaceSkills,
} from '@/hooks/useMarketplace';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

export default function MarketplacePage() {
  const { data: skills, isLoading } = useMarketplaceSkills();
  const enableSkill = useEnableMarketplaceSkill();
  const disableSkill = useDisableMarketplaceSkill();

  const marketplaceSkills = skills ?? [];

  const handleEnable = (id: string) => {
    enableSkill.mutate(id);
  };

  const handleDisable = (id: string) => {
    disableSkill.mutate(id);
  };

  return (
    <div>
      <Header
        title={t('tabMarketplace')}
        description={t('marketplacePageDescription')}
      />

      {isLoading && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-52 rounded-xl" />
          ))}
        </div>
      )}

      {!isLoading && marketplaceSkills.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="h-14 w-14 flex items-center justify-center rounded-xl bg-muted mb-4">
            <span className="text-xl text-muted-foreground">*</span>
          </div>
          <h3 className="text-sm font-semibold text-foreground mb-1.5">
            {t('marketplaceEmptyTitle')}
          </h3>
          <p className="text-sm text-muted-foreground max-w-sm">
            {t('marketplaceEmptyDescription')}
          </p>
        </div>
      )}

      {!isLoading && marketplaceSkills.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {marketplaceSkills.map((skill) => {
            const isEnabling =
              enableSkill.isPending && enableSkill.variables === skill.id;
            const isDisabling =
              disableSkill.isPending && disableSkill.variables === skill.id;
            const isBusy = isEnabling || isDisabling;

            return (
              <Card key={skill.id}>
                <CardContent className="p-5 space-y-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h3 className="text-sm font-semibold text-foreground truncate">
                        {skill.name || skill.id}
                      </h3>
                      <p className="text-xs text-muted-foreground truncate">
                        {skill.id}
                      </p>
                    </div>
                    <span
                      className={cn(
                        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium shrink-0',
                        skill.enabled
                          ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                          : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
                      )}
                    >
                      <span
                        className={cn(
                          'h-1.5 w-1.5 rounded-full',
                          skill.enabled ? 'bg-emerald-500' : 'bg-gray-400',
                        )}
                      />
                      {skill.enabled ? t('on') : t('off')}
                    </span>
                  </div>

                  <p className="text-sm text-muted-foreground min-h-[2.5rem]">
                    {skill.description || t('marketplaceNoDescription')}
                  </p>

                  <div className="space-y-1 text-xs text-muted-foreground">
                    <div>
                      {t('marketplaceVersion')}: {skill.version || '-'}
                    </div>
                    <div>
                      {t('marketplaceAuthor')}: {skill.author || '-'}
                    </div>
                    <div>
                      {t('marketplaceTags')}:{' '}
                      {skill.tags && skill.tags.length > 0
                        ? skill.tags.join(', ')
                        : '-'}
                    </div>
                    <div className="truncate" title={skill.file_path || ''}>
                      {t('marketplaceFilePath')}: {skill.file_path || '-'}
                    </div>
                  </div>

                  <div className="flex items-center gap-2 pt-1">
                    {skill.enabled ? (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={isBusy || skill.always}
                        onClick={() => handleDisable(skill.id)}
                      >
                        {isDisabling ? t('marketplaceDisabling') : t('marketplaceDisable')}
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        disabled={isBusy || skill.always}
                        onClick={() => handleEnable(skill.id)}
                      >
                        {isEnabling ? t('marketplaceEnabling') : t('marketplaceEnable')}
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
