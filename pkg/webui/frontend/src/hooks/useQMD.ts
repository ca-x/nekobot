import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';

export interface QMDCollection {
  Name: string;
  Path: string;
  Pattern: string;
  DocumentCount: number;
  LastUpdated: string;
}

export interface QMDStatusResponse {
  enabled: boolean;
  command: string;
  resolved_command: string;
  command_source: string;
  persistent_command: string;
  include_default: boolean;
  available: boolean;
  version: string;
  error: string;
  last_update: string;
  collections: QMDCollection[];
  sessions_enabled: boolean;
  session_export_dir: string;
  session_retention_days: number;
  session_export_file_count: number;
}

export interface QMDInstallResponse {
  installed: boolean;
  package: string;
  prefix: string;
  binary: string;
  command: string;
  resolved_command: string;
  command_source: string;
  persistent_command: string;
  available: boolean;
  version: string;
  error: string;
  output: string;
}

export interface QMDCleanupExportsResponse {
  deleted: number;
  remaining: number;
  session_export_dir: string;
  session_retention_days: number;
}

const qmdKeys = {
  all: ['qmd'] as const,
  status: () => [...qmdKeys.all, 'status'] as const,
};

export function useQMDStatus() {
  return useQuery<QMDStatusResponse>({
    queryKey: qmdKeys.status(),
    queryFn: () => api.get<QMDStatusResponse>('/api/memory/qmd/status'),
    staleTime: 10_000,
  });
}

export function useUpdateQMD() {
  const qc = useQueryClient();
  return useMutation<QMDStatusResponse, Error, void>({
    mutationFn: () => api.post<QMDStatusResponse>('/api/memory/qmd/update', {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qmdKeys.all });
      toast.success('QMD updated');
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useInstallQMD() {
  const qc = useQueryClient();
  return useMutation<QMDInstallResponse, Error, void>({
    mutationFn: () => api.post<QMDInstallResponse>('/api/memory/qmd/install', {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qmdKeys.all });
      toast.success('QMD installed');
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useCleanupQMDExports() {
  const qc = useQueryClient();
  return useMutation<QMDCleanupExportsResponse, Error, void>({
    mutationFn: () => api.post<QMDCleanupExportsResponse>('/api/memory/qmd/sessions/cleanup', {}),
    onSuccess: (result) => {
      qc.invalidateQueries({ queryKey: qmdKeys.all });
      toast.success(`Cleaned ${result.deleted} QMD exports`);
    },
    onError: (err) => toast.error(err.message),
  });
}
