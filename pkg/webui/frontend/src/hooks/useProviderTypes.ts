import { api } from '@/api/client';
import { useQuery } from '@tanstack/react-query';

export interface ProviderTypeField {
  key: string;
  label: string;
  type: string;
  placeholder?: string;
  required: boolean;
  secret?: boolean;
}

export interface ProviderType {
  id: string;
  display_name: string;
  icon: string;
  description: string;
  default_api_base?: string;
  supports_discovery: boolean;
  capabilities: string[];
  auth_fields: ProviderTypeField[];
  advanced_fields: ProviderTypeField[];
}

export const PROVIDER_TYPES_KEY = ['provider-types'] as const;

export function useProviderTypes() {
  return useQuery<ProviderType[]>({
    queryKey: [...PROVIDER_TYPES_KEY],
    queryFn: () => api.get('/api/provider-types'),
    staleTime: 5 * 60_000,
  });
}
