import { api, ApiError } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

export interface UserRecord {
  id: string;
  username: string;
  nickname: string;
  role: string;
  enabled: boolean;
  tenant_id: string;
  tenant_slug: string;
  last_login?: string;
  created_at: string;
  updated_at: string;
}

export interface LicenseStatus {
  install_id: string;
  licensed: boolean;
  state: string;
  max_users: number;
  free_user_limit: number;
  enabled_user_count: number;
  remaining_user_slots: number;
  license_id?: string;
  subject?: string;
  expires_at?: string;
  error?: string;
}

export interface UserInput {
  username: string;
  nickname: string;
  password?: string;
  role: string;
  enabled: boolean;
}

export class UserLimitError extends Error {
  constructor(public license?: LicenseStatus) {
    super('user_limit_reached');
    this.name = 'UserLimitError';
  }
}

const USERS_KEY = ['users'] as const;
const LICENSE_KEY = ['license', 'status'] as const;

function invalidate(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: [...USERS_KEY] });
  queryClient.invalidateQueries({ queryKey: [...LICENSE_KEY] });
}

async function withUserLimit<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (err) {
    if (err instanceof ApiError && err.status === 402) {
      try {
        const parsed = JSON.parse(err.message) as { error?: string; license?: LicenseStatus };
        if (parsed.error === 'user_limit_reached') {
          throw new UserLimitError(parsed.license);
        }
      } catch (parseErr) {
        if (parseErr instanceof UserLimitError) {
          throw parseErr;
        }
      }
    }
    throw err;
  }
}

export function useUsers() {
  return useQuery<UserRecord[]>({
    queryKey: [...USERS_KEY],
    queryFn: () => api.get('/api/users'),
    staleTime: 10_000,
  });
}

export function useLicenseStatus(enabled = false) {
  return useQuery<LicenseStatus>({
    queryKey: [...LICENSE_KEY],
    queryFn: () => api.get('/api/license/status'),
    staleTime: 10_000,
    enabled,
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UserInput) => withUserLimit(() => api.post<UserRecord>('/api/users', input)),
    onSuccess: () => invalidate(queryClient),
  });
}

export function useUpdateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UserInput }) =>
      withUserLimit(() => api.put<UserRecord>(`/api/users/${encodeURIComponent(id)}`, input)),
    onSuccess: () => invalidate(queryClient),
  });
}

export function useDeleteUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/users/${encodeURIComponent(id)}`),
    onSuccess: () => invalidate(queryClient),
  });
}

export function useImportLicense() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (license: string) => api.post<LicenseStatus>('/api/license/import', { license }),
    onSuccess: () => invalidate(queryClient),
  });
}
