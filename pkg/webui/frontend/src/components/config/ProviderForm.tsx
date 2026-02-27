import { useEffect, useState, useMemo } from 'react';
import { useForm, Controller } from 'react-hook-form';
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { MaskedInput } from '@/components/common/MaskedInput';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  useCreateProvider,
  useUpdateProvider,
  useDeleteProvider,
  useDiscoverModels,
  type Provider,
  type CreateProviderInput,
  type UpdateProviderInput,
} from '@/hooks/useProviders';
import {
  KeyRound,
  Globe,
  Search,
  Plus,
  X,
  Loader2,
  Trash2,
  Check,
} from 'lucide-react';
import { ScrollArea } from '@/components/ui/scroll-area';

// ---------- Constants ----------

const PROVIDER_TYPES = [
  'openai',
  'anthropic',
  'gemini',
  'ollama',
  'groq',
  'lmstudio',
  'vllm',
  'deepseek',
  'moonshot',
  'zhipu',
  'openrouter',
  'nvidia',
  'generic',
] as const;

type ProviderType = (typeof PROVIDER_TYPES)[number];

// ---------- Types ----------

interface ProviderFormData {
  name: string;
  provider_kind: ProviderType;
  api_key: string;
  api_base: string;
  proxy: string;
  timeout: string; // keep as string for input, parse on submit
}

interface ProviderFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  provider: Provider | null; // null = create mode
}

// ---------- Component ----------

export function ProviderForm({ open, onOpenChange, provider }: ProviderFormProps) {
  const isEdit = provider !== null;

  const createProvider = useCreateProvider();
  const updateProvider = useUpdateProvider();
  const deleteProvider = useDeleteProvider();
  const discoverModels = useDiscoverModels();

  const { register, handleSubmit, control, reset, watch, formState: { errors } } = useForm<ProviderFormData>({
    defaultValues: {
      name: '',
      provider_kind: 'openai',
      api_key: '',
      api_base: '',
      proxy: '',
      timeout: '',
    },
  });

  // Model management state
  const [models, setModels] = useState<string[]>([]);
  const [discoveredModels, setDiscoveredModels] = useState<string[]>([]);
  const [selectedDiscovered, setSelectedDiscovered] = useState<Set<string>>(new Set());
  const [modelFilter, setModelFilter] = useState('');
  const [manualModel, setManualModel] = useState('');
  const [showModelPicker, setShowModelPicker] = useState(false);

  // Populate form on open / provider change
  useEffect(() => {
    if (open) {
      if (provider) {
        reset({
          name: provider.name,
          provider_kind: (provider.provider_kind || 'openai') as ProviderType,
          api_key: '',
          api_base: provider.api_base ?? '',
          proxy: provider.proxy ?? '',
          timeout: provider.timeout ? String(provider.timeout) : '',
        });
        setModels(provider.models ?? []);
      } else {
        reset({
          name: '',
          provider_kind: 'openai',
          api_key: '',
          api_base: '',
          proxy: '',
          timeout: '',
        });
        setModels([]);
      }
      setDiscoveredModels([]);
      setSelectedDiscovered(new Set());
      setModelFilter('');
      setManualModel('');
      setShowModelPicker(false);
    }
  }, [open, provider, reset]);

  // Watched values for model discovery
  const watchedKind = watch('provider_kind');
  const watchedApiKey = watch('api_key');
  const watchedApiBase = watch('api_base');
  const watchedProxy = watch('proxy');
  const watchedTimeout = watch('timeout');

  // Filter discovered models
  const filteredDiscovered = useMemo(() => {
    if (!modelFilter.trim()) return discoveredModels;
    const lc = modelFilter.toLowerCase();
    return discoveredModels.filter((m) => m.toLowerCase().includes(lc));
  }, [discoveredModels, modelFilter]);

  // ---------- Handlers ----------

  const close = () => onOpenChange(false);

  const onSubmit = (data: ProviderFormData) => {
    if (isEdit) {
      const payload: UpdateProviderInput = {
        provider_kind: data.provider_kind,
        api_base: data.api_base || undefined,
        proxy: data.proxy || undefined,
        timeout: data.timeout ? parseInt(data.timeout, 10) : undefined,
        models,
      };
      if (data.api_key) payload.api_key = data.api_key;
      updateProvider.mutate(
        { name: provider!.name, data: payload },
        { onSuccess: close },
      );
    } else {
      if (!data.name.trim()) return;
      const payload: CreateProviderInput = {
        name: data.name.trim(),
        provider_kind: data.provider_kind,
        api_key: data.api_key || undefined,
        api_base: data.api_base || undefined,
        proxy: data.proxy || undefined,
        timeout: data.timeout ? parseInt(data.timeout, 10) : undefined,
      };
      createProvider.mutate(payload, { onSuccess: close });
    }
  };

  const handleDelete = () => {
    if (!provider) return;
    if (!window.confirm(t('deleteConfirm'))) return;
    deleteProvider.mutate(provider.name, { onSuccess: close });
  };

  const handleDiscover = () => {
    discoverModels.mutate(
      {
        name: provider?.name,
        provider_kind: watchedKind,
        api_key: watchedApiKey || provider?.api_key,
        api_base: watchedApiBase || provider?.api_base,
        proxy: watchedProxy || provider?.proxy,
        timeout: watchedTimeout ? parseInt(watchedTimeout, 10) : provider?.timeout,
      },
      {
        onSuccess: (resp) => {
          const discovered = resp.models ?? [];
          setDiscoveredModels(discovered);
          // Pre-select already-configured models
          const existing = new Set(models);
          const preSelected = new Set(discovered.filter((m) => existing.has(m)));
          setSelectedDiscovered(preSelected);
          setShowModelPicker(true);
        },
      },
    );
  };

  const toggleDiscoveredModel = (model: string) => {
    setSelectedDiscovered((prev) => {
      const next = new Set(prev);
      if (next.has(model)) next.delete(model);
      else next.add(model);
      return next;
    });
  };

  const selectAllVisible = () => {
    setSelectedDiscovered((prev) => {
      const next = new Set(prev);
      filteredDiscovered.forEach((m) => next.add(m));
      return next;
    });
  };

  const clearSelection = () => {
    setSelectedDiscovered(new Set());
  };

  const applyDiscoveredSelection = () => {
    // Merge: keep manually added models that are not in discovered list, plus selected discovered models
    const discoveredSet = new Set(discoveredModels);
    const manuallyAdded = models.filter((m) => !discoveredSet.has(m));
    const merged = [...new Set([...manuallyAdded, ...selectedDiscovered])];
    setModels(merged);
    setShowModelPicker(false);
  };

  const addManualModel = () => {
    const name = manualModel.trim();
    if (!name) return;
    if (!models.includes(name)) {
      setModels((prev) => [...prev, name]);
    }
    setManualModel('');
  };

  const removeModel = (model: string) => {
    setModels((prev) => prev.filter((m) => m !== model));
  };

  const isSaving = createProvider.isPending || updateProvider.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[560px] max-h-[90vh] flex flex-col">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-primary flex items-center justify-center shrink-0">
              <KeyRound className="h-5 w-5 text-primary-foreground" />
            </div>
            <div>
              <DialogTitle>
                {isEdit ? t('editProviderDialogTitle') : t('newProviderDialogTitle')}
              </DialogTitle>
              <DialogDescription>
                {isEdit ? provider!.name : t('creatingNew')}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col flex-1 min-h-0">
          <ScrollArea className="flex-1 pr-3 -mr-3">
            <div className="space-y-5 pb-2">
              {/* Provider Name */}
              <div className="space-y-2">
                <Label htmlFor="pf-name">{t('providerName')}</Label>
                <Input
                  id="pf-name"
                  placeholder="e.g. my-openai\u2026"
                  disabled={isEdit}
                  {...register('name', { required: !isEdit })}
                  className={cn(errors.name && 'border-destructive')}
                />
              </div>

              {/* Provider Type */}
              <div className="space-y-2">
                <Label htmlFor="pf-kind">{t('providerType')}</Label>
                <Controller
                  name="provider_kind"
                  control={control}
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <SelectTrigger id="pf-kind">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {PROVIDER_TYPES.map((pt) => (
                          <SelectItem key={pt} value={pt}>
                            {pt}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>

              {/* API Endpoint */}
              <div className="space-y-2">
                <Label htmlFor="pf-endpoint" className="flex items-center gap-1.5">
                  <Globe className="h-3.5 w-3.5 text-muted-foreground" />
                  {t('apiEndpoint')}
                </Label>
                <Input
                  id="pf-endpoint"
                  placeholder="https://api.openai.com/v1"
                  {...register('api_base')}
                />
              </div>

              {/* API Key */}
              <div className="space-y-2">
                <Label htmlFor="pf-key" className="flex items-center gap-1.5">
                  <KeyRound className="h-3.5 w-3.5 text-muted-foreground" />
                  {t('apiKey')}
                </Label>
                <Controller
                  name="api_key"
                  control={control}
                  render={({ field }) => (
                    <MaskedInput
                      id="pf-key"
                      value={field.value}
                      onChange={field.onChange}
                      isSet={isEdit && !!provider?.api_key}
                      placeholder={
                        isEdit && provider?.api_key
                          ? 'sk-****  (leave blank to keep current)'
                          : 'sk-\u2026'
                      }
                    />
                  )}
                />
              </div>

              {/* Proxy Address */}
              <div className="space-y-2">
                <Label htmlFor="pf-proxy">{t('proxyAddress')}</Label>
                <Input
                  id="pf-proxy"
                  placeholder="http://127.0.0.1:7890"
                  {...register('proxy')}
                />
              </div>

              {/* Timeout */}
              <div className="space-y-2">
                <Label htmlFor="pf-timeout">Timeout (s)</Label>
                <Input
                  id="pf-timeout"
                  type="number"
                  min={0}
                  placeholder="30"
                  {...register('timeout')}
                />
              </div>

              {/* Models Section */}
              {isEdit && (
                <div className="space-y-3 pt-2 border-t">
                  <div className="flex items-center justify-between">
                    <Label className="text-sm font-medium">
                      Models ({models.length})
                    </Label>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={discoverModels.isPending}
                      onClick={handleDiscover}
                    >
                      {discoverModels.isPending ? (
                        <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                      ) : (
                        <Search className="h-4 w-4 mr-1.5" />
                      )}
                      {discoverModels.isPending
                        ? t('discoveringModels')
                        : t('fetchModels')}
                    </Button>
                  </div>

                  {/* Model Picker (discovered) */}
                  {showModelPicker && discoveredModels.length > 0 && (
                    <div className="border rounded-lg p-3 space-y-2 bg-muted/30">
                      <div className="flex items-center gap-2">
                        <Input
                          type="text"
                          placeholder={t('filterModels')}
                          value={modelFilter}
                          onChange={(e) => setModelFilter(e.target.value)}
                          className="h-8 text-sm"
                        />
                      </div>
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span>
                          {t(
                            'providerModelsSelected',
                            String(selectedDiscovered.size),
                            String(discoveredModels.length),
                            String(filteredDiscovered.length),
                          )}
                        </span>
                        <button
                          type="button"
                          className="text-primary hover:underline"
                          onClick={selectAllVisible}
                        >
                          {t('selectAllModels')}
                        </button>
                        <button
                          type="button"
                          className="text-primary hover:underline"
                          onClick={clearSelection}
                        >
                          {t('clearModelSelection')}
                        </button>
                      </div>
                      <ScrollArea className="max-h-48">
                        <div className="space-y-0.5">
                          {filteredDiscovered.map((model) => {
                            const checked = selectedDiscovered.has(model);
                            return (
                              <button
                                key={model}
                                type="button"
                                className={cn(
                                  'flex items-center gap-2 w-full text-left px-2 py-1 rounded text-sm hover:bg-accent transition-colors',
                                  checked && 'bg-accent',
                                )}
                                onClick={() => toggleDiscoveredModel(model)}
                              >
                                <div
                                  className={cn(
                                    'h-4 w-4 rounded border flex items-center justify-center shrink-0',
                                    checked
                                      ? 'bg-primary border-primary text-primary-foreground'
                                      : 'border-input',
                                  )}
                                >
                                  {checked && <Check className="h-3 w-3" />}
                                </div>
                                <span className="truncate">{model}</span>
                              </button>
                            );
                          })}
                        </div>
                      </ScrollArea>
                      <Button
                        type="button"
                        size="sm"
                        onClick={applyDiscoveredSelection}
                        className="w-full"
                      >
                        {t('applyModelSelection')}
                      </Button>
                    </div>
                  )}

                  {/* Manual model add */}
                  <div className="flex gap-2">
                    <Input
                      type="text"
                      placeholder={t('manualModel')}
                      value={manualModel}
                      onChange={(e) => setManualModel(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault();
                          addManualModel();
                        }
                      }}
                      className="flex-1"
                    />
                    <Button type="button" variant="outline" size="icon" onClick={addManualModel} aria-label={t('add')}>
                      <Plus className="h-4 w-4" />
                    </Button>
                  </div>

                  {/* Current models list */}
                  {models.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                      {models.map((model) => (
                        <span
                          key={model}
                          className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-secondary text-secondary-foreground text-xs"
                        >
                          <span className="truncate max-w-[200px]">{model}</span>
                          <button
                            type="button"
                            className="text-muted-foreground hover:text-foreground"
                            onClick={() => removeModel(model)}
                          >
                            <X className="h-3 w-3" />
                          </button>
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          </ScrollArea>

          <DialogFooter className="pt-4 flex-shrink-0 gap-2">
            {isEdit && (
              <Button
                type="button"
                variant="destructive"
                onClick={handleDelete}
                disabled={deleteProvider.isPending}
                className="mr-auto"
              >
                <Trash2 className="h-4 w-4 mr-1.5" />
                {t('delete')}
              </Button>
            )}
            <Button type="button" variant="outline" onClick={close}>
              {t('cancel')}
            </Button>
            <Button type="submit" disabled={isSaving}>
              {isSaving ? (
                <>
                  <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
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
  );
}
