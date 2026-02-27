import { api } from '@/api/client';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

/* ---------- types ---------- */

export interface ToolSession {
  id: string;
  tool: string;
  title: string;
  command: string;
  workdir: string;
  state: string; // "running" | "terminated" | "archived" | ...
  access_mode: string; // "none" | "one_time" | "permanent"
  source?: string; // "agent" | "channel" | ""
  metadata?: Record<string, unknown>;
  created_at?: string;
  updated_at?: string;
}

export interface CreateToolSessionPayload {
  tool: string;
  title?: string;
  command_args?: string;
  workdir?: string;
  proxy_mode?: string;
  proxy_url?: string;
  access_mode?: string;
  access_password?: string;
  public_base_url?: string;
}

export interface UpdateToolSessionPayload extends CreateToolSessionPayload {}

export interface CreateSessionResponse {
  session: ToolSession;
  access_url?: string;
  access_password?: string;
  access_mode?: string;
}

export interface AccessResponse {
  access_url: string;
  access_password: string;
  access_mode: string;
}

export interface OTPResponse {
  otp_code: string;
  expires_at: number;
  ttl_seconds: number;
}

export interface ProcessStatus {
  running: boolean;
  exit_code?: number;
  missing?: boolean;
}

/* ---------- query keys ---------- */

export const toolSessionKeys = {
  all: ['tool-sessions'] as const,
  list: () => [...toolSessionKeys.all, 'list'] as const,
  detail: (id: string) => [...toolSessionKeys.all, 'detail', id] as const,
  processStatus: (id: string) => [...toolSessionKeys.all, 'process-status', id] as const,
};

/* ---------- queries ---------- */

export function useToolSessions() {
  return useQuery<ToolSession[]>({
    queryKey: toolSessionKeys.list(),
    queryFn: async () => {
      const data = await api.get<ToolSession[]>('/api/tool-sessions?limit=200');
      return Array.isArray(data) ? data : [];
    },
    staleTime: 5_000,
    refetchInterval: 10_000,
  });
}

export function useProcessStatus(sessionId: string | null) {
  return useQuery<ProcessStatus>({
    queryKey: toolSessionKeys.processStatus(sessionId ?? ''),
    queryFn: () =>
      api.get<ProcessStatus>(
        `/api/tool-sessions/${encodeURIComponent(sessionId!)}/process/status`,
      ),
    enabled: !!sessionId,
    staleTime: 3_000,
    refetchInterval: 5_000,
  });
}

/* ---------- mutations ---------- */

export function useCreateToolSession() {
  const qc = useQueryClient();
  return useMutation<CreateSessionResponse, Error, CreateToolSessionPayload>({
    mutationFn: (payload) => api.post('/api/tool-sessions', payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: toolSessionKeys.list() });
    },
    onError: (err) => toast.error(err.message || t('createSessionFailed')),
  });
}

export function useUpdateToolSession() {
  const qc = useQueryClient();
  return useMutation<
    CreateSessionResponse,
    Error,
    { id: string; payload: UpdateToolSessionPayload }
  >({
    mutationFn: ({ id, payload }) =>
      api.put(`/api/tool-sessions/${encodeURIComponent(id)}`, payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: toolSessionKeys.list() });
      toast.success(t('saved'));
    },
    onError: (err) => toast.error(err.message || t('saveSessionFailed')),
  });
}

export function useRestartToolSession() {
  const qc = useQueryClient();
  return useMutation<CreateSessionResponse, Error, string>({
    mutationFn: (id) =>
      api.post(`/api/tool-sessions/${encodeURIComponent(id)}/restart`, {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: toolSessionKeys.list() });
    },
    onError: (err) => toast.error(err.message || t('restartSessionFailed')),
  });
}

export function useDetachToolSession() {
  const qc = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      api.post(`/api/tool-sessions/${encodeURIComponent(id)}/detach`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: toolSessionKeys.list() });
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useTerminateToolSession() {
  const qc = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      api.post(`/api/tool-sessions/${encodeURIComponent(id)}/terminate`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: toolSessionKeys.list() });
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useCleanupTerminated() {
  const qc = useQueryClient();
  return useMutation<void, Error, void>({
    mutationFn: () => api.post('/api/tool-sessions/cleanup-terminated'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: toolSessionKeys.list() });
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useKillToolProcess() {
  const qc = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      api.post(`/api/tool-sessions/${encodeURIComponent(id)}/process/kill`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: toolSessionKeys.list() });
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useUpdateAccess() {
  return useMutation<
    AccessResponse,
    Error,
    { id: string; mode: string; password?: string }
  >({
    mutationFn: ({ id, mode, password }) =>
      api.post(`/api/tool-sessions/${encodeURIComponent(id)}/access`, {
        mode,
        password: password || '',
      }),
    onError: (err) => toast.error(err.message || t('accessNotAvailable')),
  });
}

export function useGenerateOTP() {
  return useMutation<OTPResponse, Error, string>({
    mutationFn: (id) =>
      api.post(`/api/tool-sessions/${encodeURIComponent(id)}/otp`, {}),
    onError: (err) => toast.error(err.message || t('otpUnavailable')),
  });
}
