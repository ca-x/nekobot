import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogPortal,
  DialogTitle,
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
import { Switch } from '@/components/ui/switch';
import { MaskedInput } from '@/components/common/MaskedInput';
import { ScrollArea } from '@/components/ui/scroll-area';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { toast } from '@/lib/notify';
import { getProviderLogo } from '@/lib/provider-logos';
import {
  useApplyDiscoveredModels,
  useCreateProvider,
  useDeleteProvider,
  useDiscoverModels,
  useUpdateProvider,
  type CreateProviderInput,
  type Provider,
  type UpdateProviderInput,
} from '@/hooks/useProviders';
import { useProviderTypes } from '@/hooks/useProviderTypes';
import {
  CheckCircle2,
  Globe,
  KeyRound,
  Loader2,
  Search,
  ShieldCheck,
  Trash2,
} from 'lucide-react';

interface ProviderFormData {
  name: string;
  provider_kind: string;
  api_key: string;
  api_base: string;
  proxy: string;
  timeout: string;
  default_weight: string;
  default_test_model: string;
  api_format: string;
  enabled: boolean;
}

interface ProviderFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  provider: Provider | null;
}

const API_FORMAT_OPTIONS = [
  { value: 'openai/chat_completions', label: 'Chat Completions' },
  { value: 'openai/responses', label: 'Responses' },
] as const;

const PROVIDER_KINDS_REQUIRING_API_KEY = new Set([
  'openai',
  'anthropic',
  'gemini',
  'openrouter',
]);

function toFormData(provider: Provider | null): ProviderFormData {
  return {
    name: provider?.name ?? '',
    provider_kind: provider?.provider_kind ?? 'openai',
    api_key: '',
    api_base: provider?.api_base ?? '',
    proxy: provider?.proxy ?? '',
    timeout: provider?.timeout ? String(provider.timeout) : '',
    default_weight: String(provider?.default_weight ?? 1),
    default_test_model: provider?.default_test_model ?? '',
    api_format: provider?.api_format || 'openai/chat_completions',
    enabled: provider?.enabled ?? true,
  };
}

export function ProviderForm({ open, onOpenChange, provider }: ProviderFormProps) {
  const isEdit = provider !== null;
  const { data: providerTypes = [] } = useProviderTypes();
  const createProvider = useCreateProvider();
  const updateProvider = useUpdateProvider();
  const deleteProvider = useDeleteProvider();
  const discoverModels = useDiscoverModels();
  const applyDiscoveredModels = useApplyDiscoveredModels();
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [discoveredModels, setDiscoveredModels] = useState<string[]>([]);
  const [selectedDiscoveredModels, setSelectedDiscoveredModels] = useState<string[]>([]);
  const [discoveredModelQuery, setDiscoveredModelQuery] = useState('');

  const {
    register,
    handleSubmit,
    control,
    reset,
    watch,
    setValue,
    formState: { errors },
  } = useForm<ProviderFormData>({
    defaultValues: toFormData(provider),
  });

  useEffect(() => {
    if (!open) {
      return;
    }
    reset(toFormData(provider));
    setDiscoveredModels([]);
    setSelectedDiscoveredModels([]);
    setDiscoveredModelQuery('');
  }, [open, provider, reset]);

  const selectedKind = watch('provider_kind');
  const draftName = watch('name');
  const draftAPIKey = watch('api_key');
  const selectedAPIFormat = watch('api_format');
  const selectedType = useMemo(
    () => providerTypes.find((item) => item.id === selectedKind) ?? providerTypes[0] ?? null,
    [providerTypes, selectedKind],
  );

  useEffect(() => {
    if (!selectedType) {
      return;
    }
    const current = watch('api_base');
    if (!current.trim() && selectedType.default_api_base) {
      setValue('api_base', selectedType.default_api_base, { shouldDirty: !isEdit });
    }
  }, [isEdit, selectedType, setValue, watch]);

  const close = () => onOpenChange(false);
  const isSaving = createProvider.isPending || updateProvider.isPending;
  const requiredAuthFields = selectedType?.auth_fields ?? [];
  const apiKeyRequired =
    requiredAuthFields.some((field) => field.key === 'api_key' && field.required) ||
    PROVIDER_KINDS_REQUIRING_API_KEY.has(selectedKind);
  const hasExistingRequiredApiKey = Boolean(isEdit && provider?.api_key_set);
  const hasSatisfiedRequiredApiKey = !apiKeyRequired || hasExistingRequiredApiKey || Boolean(draftAPIKey.trim());
  const saveDisabled =
    isSaving ||
    (!isEdit && !draftName.trim()) ||
    !hasSatisfiedRequiredApiKey;
  const discoverDisabled =
    discoverModels.isPending ||
    !hasSatisfiedRequiredApiKey;
  const applyDiscoveredDisabled =
    applyDiscoveredModels.isPending ||
    selectedDiscoveredModels.length === 0 ||
    !(provider?.name || draftName.trim());
  const logo = getProviderLogo(selectedType?.icon ?? selectedKind);

  const defaultTestModelOptions = useMemo(() => {
    return Array.from(new Set(selectedDiscoveredModels.length > 0 ? selectedDiscoveredModels : discoveredModels));
  }, [discoveredModels, selectedDiscoveredModels]);

  const filteredDiscoveredModels = useMemo(() => {
    const keyword = discoveredModelQuery.trim().toLowerCase();
    if (!keyword) {
      return discoveredModels;
    }
    return discoveredModels.filter((modelID) => modelID.toLowerCase().includes(keyword));
  }, [discoveredModelQuery, discoveredModels]);

  const applyDiscover = () => {
    const values = watch();
    discoverModels.mutate(
      {
        name: provider?.name || values.name.trim() || undefined,
        provider_kind: values.provider_kind,
        api_key: values.api_key || undefined,
        api_base: values.api_base || undefined,
        proxy: values.proxy || undefined,
        timeout: values.timeout ? Number(values.timeout) : undefined,
      },
      {
        onSuccess: (result) => {
          setDiscoveredModels(result.models);
          setSelectedDiscoveredModels(result.models);
          toast.success(t('providerDiscoveredModelsPreviewReady', String(result.models.length)));
        },
      },
    );
  };

  const toggleDiscoveredModel = (modelID: string) => {
    setSelectedDiscoveredModels((current) =>
      current.includes(modelID)
        ? current.filter((item) => item !== modelID)
        : [...current, modelID],
    );
  };

  const applySelectedModels = () => {
    const values = watch();
    const providerName = provider?.name || values.name.trim();
    if (!providerName) {
      toast.error(t('providerDiscoverSaveProviderFirst'));
      return;
    }
    if (!values.default_test_model.trim() && selectedDiscoveredModels.length > 0) {
      setValue('default_test_model', selectedDiscoveredModels[0], { shouldDirty: true });
    }
    applyDiscoveredModels.mutate({
      profile: {
        name: providerName,
        provider_kind: values.provider_kind,
      },
      models: selectedDiscoveredModels,
    });
  };

  const onSubmit = (data: ProviderFormData) => {
    const base = {
      provider_kind: data.provider_kind,
      api_base: data.api_base.trim() || undefined,
      proxy: data.proxy.trim() || undefined,
      timeout: data.timeout.trim() ? Number(data.timeout) : undefined,
      default_weight: data.default_weight.trim() ? Number(data.default_weight) : 1,
      default_test_model: data.default_test_model.trim() || undefined,
      api_format: data.api_format.trim() || 'openai/chat_completions',
      enabled: data.enabled,
    };

    if (isEdit) {
      const payload: UpdateProviderInput = {
        ...base,
      };
      if (data.api_key.trim()) {
        payload.api_key = data.api_key.trim();
      }
      updateProvider.mutate(
        { name: provider.name, data: payload },
        { onSuccess: close },
      );
      return;
    }

    const payload: CreateProviderInput = {
      name: data.name.trim(),
      ...base,
      api_key: data.api_key.trim() || undefined,
    };
    createProvider.mutate(payload, { onSuccess: close });
  };

  const confirmDelete = () => {
    if (!provider) {
      return;
    }
    deleteProvider.mutate(provider.name, {
      onSuccess: () => {
        setShowDeleteConfirm(false);
        close();
      },
    });
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="flex max-h-[90vh] flex-col overflow-hidden sm:max-w-[760px]">
          <DialogHeader>
            <div className="flex items-start gap-4">
              <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[20px] border border-border/70 bg-card shadow-sm">
                {logo ? (
                  <img src={logo} alt={selectedKind} className="h-8 w-8 object-contain" />
                ) : (
                  <KeyRound className="h-6 w-6 text-primary" />
                )}
              </div>
              <div className="space-y-1">
                <DialogTitle>{isEdit ? t('editProviderDialogTitle') : t('newProviderDialogTitle')}</DialogTitle>
                <DialogDescription>
                  {isEdit ? provider.name : t('providerDialogCreateFirst')}
                </DialogDescription>
              </div>
            </div>
          </DialogHeader>

          <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
            <ScrollArea className="flex-1 pr-3">
              <div className="space-y-6 pb-3">
                <section className="space-y-3">
                  <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                    <ShieldCheck className="h-3.5 w-3.5" />
                    {t('providerType')}
                  </div>
                  <Controller
                    name="provider_kind"
                    control={control}
                    render={({ field }) => (
                      <Select value={field.value} onValueChange={field.onChange} disabled={providerTypes.length === 0}>
                        <SelectTrigger className="h-11 rounded-2xl bg-card/90">
                          <SelectValue placeholder={t('providerType')} />
                        </SelectTrigger>
                        <SelectContent>
                          {providerTypes.map((item) => (
                            <SelectItem key={item.id} value={item.id}>
                              {item.display_name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                  />

                  {selectedType && (
                    <div className="grid gap-3 sm:grid-cols-3">
                      <div className="rounded-2xl border border-border/70 bg-card/85 p-4 sm:col-span-2">
                        <div className="text-sm font-semibold text-foreground">{selectedType.display_name}</div>
                        <p className="mt-2 text-sm leading-6 text-muted-foreground">{selectedType.description}</p>
                        <div className="mt-3 flex flex-wrap gap-2">
                          {selectedType.capabilities.map((capability) => (
                            <span
                              key={capability}
                              className="rounded-full border border-border/70 bg-background px-2.5 py-1 text-xs text-muted-foreground"
                            >
                              {capability}
                            </span>
                          ))}
                        </div>
                      </div>
                      <div className="rounded-2xl border border-border/70 bg-card/85 p-4">
                        <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground">{t('defaults')}</div>
                        <div className="mt-3 space-y-3 text-sm">
                          <div>
                            <div className="text-muted-foreground">{t('apiEndpoint')}</div>
                            <div className="mt-1 break-all text-foreground">
                              {selectedType.default_api_base || t('providerPanelManualOnly')}
                            </div>
                          </div>
                          <div>
                            <div className="text-muted-foreground">{t('providerDiscoverLabel')}</div>
                            <div className="mt-1 text-foreground">
                              {selectedType.supports_discovery ? t('providerDiscoverySupported') : t('providerPanelManualOnly')}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </section>

                <section className="grid gap-4 xl:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="pf-name">{t('providerName')}</Label>
                    <Input
                      id="pf-name"
                      placeholder={t('providerNamePlaceholder')}
                      disabled={isEdit}
                      {...register('name', { required: !isEdit })}
                      className={cn('h-11 rounded-2xl bg-card/90', errors.name && 'border-destructive')}
                    />
                    {errors.name && <p className="text-xs text-destructive">{t('providerNameRequired')}</p>}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="pf-api-key">{t('apiKey')}</Label>
                    <Controller
                      name="api_key"
                      control={control}
                      render={({ field }) => (
                        <MaskedInput
                          id="pf-api-key"
                          value={field.value}
                          onChange={field.onChange}
                          isSet={isEdit && provider?.api_key_set}
                          placeholder={isEdit && provider?.api_key_set ? 'Leave blank to keep current value' : 'sk-...'}
                          className="h-11 rounded-2xl bg-card/90"
                        />
                      )}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="pf-api-base" className="flex items-center gap-2">
                      <Globe className="h-3.5 w-3.5 text-muted-foreground" />
                      {t('apiEndpoint')}
                    </Label>
                    <Input
                      id="pf-api-base"
                      placeholder={selectedType?.default_api_base || t('apiEndpointExample')}
                      {...register('api_base')}
                      className="h-11 rounded-2xl bg-card/90"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="pf-proxy">{t('proxyAddress')}</Label>
                    <Input
                      id="pf-proxy"
                      placeholder={t('proxyAddressPlaceholder')}
                      {...register('proxy')}
                      className="h-11 rounded-2xl bg-card/90"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="pf-timeout">{t('providerPanelTimeout')} (s)</Label>
                    <Input
                      id="pf-timeout"
                      type="number"
                      min={0}
                      {...register('timeout')}
                      className="h-11 rounded-2xl bg-card/90"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="pf-weight">{t('modelsFieldWeightOverride')}</Label>
                    <Input
                      id="pf-weight"
                      type="number"
                      min={1}
                      {...register('default_weight')}
                      className="h-11 rounded-2xl bg-card/90"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label>{t('providerDiscoverLabel')} API</Label>
                    <Controller
                      name="api_format"
                      control={control}
                      render={({ field }) => (
                        <Select value={field.value} onValueChange={field.onChange}>
                          <SelectTrigger className="h-11 rounded-2xl bg-card/90">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {API_FORMAT_OPTIONS.map((item) => (
                              <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      )}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="pf-default-test-model">{t('modelsFieldModelId')}</Label>
                    {defaultTestModelOptions.length > 0 ? (
                      <Controller
                        name="default_test_model"
                        control={control}
                        render={({ field }) => (
                          <Select value={field.value || '__unset__'} onValueChange={(value) => field.onChange(value === '__unset__' ? '' : value)}>
                            <SelectTrigger className="h-11 rounded-2xl bg-card/90">
                              <SelectValue placeholder={t('cronModelDefault')} />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="__unset__">{t('cronModelDefault')}</SelectItem>
                              {defaultTestModelOptions.map((modelID) => (
                                <SelectItem key={modelID} value={modelID}>{modelID}</SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        )}
                      />
                    ) : (
                      <Input
                        id="pf-default-test-model"
                        placeholder={t('providerDiscoverApply')}
                        {...register('default_test_model')}
                        className="h-11 rounded-2xl bg-card/90"
                      />
                    )}
                  </div>
                </section>

                <section className="rounded-[24px] border border-border/70 bg-card/70 p-4">
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <div className="space-y-1">
                      <div className="text-sm font-semibold text-foreground">{t('providerConnectionStateTitle')}</div>
                      <p className="text-sm leading-6 text-muted-foreground">
                        {t('providerConnectionStateDescription')}
                      </p>
                    </div>
                    <Controller
                      name="enabled"
                      control={control}
                      render={({ field }) => (
                        <div className="flex items-center gap-3 rounded-full border border-border/70 bg-background px-3 py-2">
                          <span className="text-sm text-muted-foreground">{t('enabled')}</span>
                          <Switch checked={field.value} onCheckedChange={field.onChange} />
                        </div>
                      )}
                    />
                  </div>

                  {selectedType?.supports_discovery && (
                    <div className="mt-4 flex flex-col gap-3 rounded-2xl border border-dashed border-border/70 bg-background/80 p-4 sm:flex-row sm:items-center sm:justify-between">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                          <Search className="h-4 w-4" />
                          {t('fetchModels')}
                        </div>
                        <p className="text-sm leading-6 text-muted-foreground">
                          {t('providerDiscoverDescription')}
                        </p>
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        className="rounded-full"
                        onClick={applyDiscover}
                        disabled={discoverDisabled}
                      >
                        {discoverModels.isPending ? (
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        ) : (
                          <Search className="mr-2 h-4 w-4" />
                        )}
                        {discoverModels.isPending ? t('discoveringModels') : t('fetchModels')}
                      </Button>
                    </div>
                  )}

                  {selectedType?.supports_discovery && discoveredModels.length > 0 && (
                    <div className="mt-4 space-y-3 rounded-2xl border border-border/70 bg-background/80 p-4">
                      <div className="space-y-1">
                        <div className="text-sm font-semibold text-foreground">{t('providerDiscoverTitle')}</div>
                        <p className="text-sm leading-6 text-muted-foreground">{t('providerDiscoverSelectionHint')}</p>
                        <p className="text-xs text-muted-foreground">{selectedAPIFormat === 'openai/responses' ? 'Responses API enabled for this provider.' : 'Chat Completions API enabled for this provider.'}</p>
                      </div>

                      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                        <div className="relative w-full lg:max-w-sm">
                          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                          <Input
                            value={discoveredModelQuery}
                            onChange={(event) => setDiscoveredModelQuery(event.target.value)}
                            placeholder={t('modelsSearchPlaceholder')}
                            className="h-10 rounded-xl bg-card pl-9"
                          />
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            type="button"
                            variant="outline"
                            className="rounded-full"
                            onClick={() => setSelectedDiscoveredModels(discoveredModels)}
                          >
                            {t('selectAll')}
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            className="rounded-full"
                            onClick={() => setSelectedDiscoveredModels([])}
                          >
                            {t('clear')}
                          </Button>
                        </div>
                      </div>

                      <ScrollArea className="h-64 rounded-2xl border border-border/70 bg-card/70">
                        <div className="space-y-2 p-3">
                          {filteredDiscoveredModels.map((modelID) => {
                            const selected = selectedDiscoveredModels.includes(modelID);
                            return (
                              <button
                                key={modelID}
                                type="button"
                                onClick={() => toggleDiscoveredModel(modelID)}
                                className={cn(
                                  'flex w-full items-center justify-between gap-3 rounded-xl border px-3 py-2 text-left text-sm transition-colors',
                                  selected
                                    ? 'border-[hsl(var(--brand-300))] bg-[hsl(var(--brand-100))]/80 text-[hsl(var(--brand-900))]'
                                    : 'border-border/70 bg-background text-foreground hover:border-border hover:bg-muted/50',
                                )}
                              >
                                <span className="min-w-0 flex-1 break-all font-mono text-xs sm:text-sm">{modelID}</span>
                                {selected ? <CheckCircle2 className="h-4 w-4 shrink-0" /> : null}
                              </button>
                            );
                          })}
                          {filteredDiscoveredModels.length === 0 ? (
                            <div className="rounded-xl border border-dashed border-border/70 px-3 py-6 text-center text-sm text-muted-foreground">
                              {t('modelsEmptyTitle')}
                            </div>
                          ) : null}
                        </div>
                      </ScrollArea>

                      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <p className="text-xs text-muted-foreground">
                          {t('providerDiscoveredModelsSelected', String(selectedDiscoveredModels.length), String(discoveredModels.length))}
                        </p>
                        <Button
                          type="button"
                          className="rounded-full"
                          onClick={applySelectedModels}
                          disabled={applyDiscoveredDisabled}
                        >
                          {applyDiscoveredModels.isPending ? (
                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                          ) : (
                            <CheckCircle2 className="mr-2 h-4 w-4" />
                          )}
                          {t('providerDiscoverApply')}
                        </Button>
                      </div>
                    </div>
                  )}

                  {!selectedType?.supports_discovery && (
                    <div className="mt-4 flex items-start gap-3 rounded-2xl border border-border/70 bg-background/80 p-4">
                      <CheckCircle2 className="mt-0.5 h-4 w-4 text-muted-foreground" />
                      <p className="text-sm leading-6 text-muted-foreground">
                        {t('providerDiscoveryManualDescription')}
                      </p>
                    </div>
                  )}
                </section>
              </div>
            </ScrollArea>

            <DialogFooter className="gap-2 pt-4">
              {isEdit && (
                <Button
                  type="button"
                  variant="destructive"
                  onClick={() => setShowDeleteConfirm(true)}
                  disabled={deleteProvider.isPending}
                  className="mr-auto"
                >
                  <Trash2 className="mr-1.5 h-4 w-4" />
                  {t('delete')}
                </Button>
              )}
              <Button type="button" variant="outline" onClick={close}>
                {t('cancel')}
              </Button>
              <Button type="submit" disabled={saveDisabled}>
                {isSaving ? (
                  <>
                    <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                    {t('saving')}
                  </>
                ) : (
                  t('save')
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogPortal>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('deleteConfirmTitle')}</DialogTitle>
              <DialogDescription>{t('deleteConfirmDescription')}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>
                {t('cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmDelete} disabled={deleteProvider.isPending}>
                {deleteProvider.isPending ? (
                  <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                ) : (
                  <Trash2 className="mr-1.5 h-4 w-4" />
                )}
                {t('delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </DialogPortal>
      </Dialog>
    </>
  );
}
