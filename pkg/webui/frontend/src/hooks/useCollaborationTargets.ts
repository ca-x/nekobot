import { useMemo } from 'react';

import { useChannels } from '@/hooks/useChannels';
import type { NotificationRoute } from '@/hooks/useNotificationRoutes';
import { useRuntimeAgents } from '@/hooks/useTopology';
import { useThreads, type ThreadSummary } from '@/hooks/useThreads';
import { t } from '@/lib/i18n';

export type CollaborationTargetKind = 'channel' | 'thread' | 'dm' | 'route' | 'unknown';

export interface CollaborationTargetOption {
  target: string;
  kind: CollaborationTargetKind;
  label: string;
  description?: string;
  source: 'channel' | 'thread' | 'runtime' | 'notification-route' | 'synthetic' | 'raw';
  disabled?: boolean;
}

export interface RouteTargetPreview {
  target: string;
  title: string;
  detail: string;
  config: Record<string, unknown>;
  parseError?: string;
}

function normalizeTarget(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return '';
  }
  if (trimmed.startsWith('#') || trimmed.startsWith('dm:')) {
    return trimmed;
  }
  return `#${trimmed}`;
}

function threadTarget(thread: Pick<ThreadSummary, 'id' | 'target'>): string {
  return thread.target?.trim() || `#websocket:${thread.id}`;
}

function addOption(targets: Map<string, CollaborationTargetOption>, option: CollaborationTargetOption) {
  const target = normalizeTarget(option.target);
  if (!target || targets.has(target)) {
    return;
  }
  targets.set(target, { ...option, target });
}

function kindFromTarget(target: string): CollaborationTargetKind {
  if (target.startsWith('dm:')) {
    return 'dm';
  }
  if (target.includes(':')) {
    return 'thread';
  }
  if (target.startsWith('#')) {
    return 'channel';
  }
  return 'unknown';
}

export function useCollaborationTargets() {
  const channelsQuery = useChannels();
  const threadsQuery = useThreads();
  const runtimesQuery = useRuntimeAgents();

  const options = useMemo(() => {
    const targets = new Map<string, CollaborationTargetOption>();

    addOption(targets, {
      target: '#websocket',
      kind: 'channel',
      label: t('collaborationTargetWebsocket'),
      description: '#websocket',
      source: 'synthetic',
    });

    for (const instance of channelsQuery.data?._instances ?? []) {
      addOption(targets, {
        target: instance.id,
        kind: 'channel',
        label: instance.name || instance.id,
        description: `${instance.type || t('collaborationTargetChannel')} · ${normalizeTarget(instance.id)}`,
        source: 'channel',
        disabled: !instance.enabled,
      });
    }

    for (const thread of threadsQuery.data ?? []) {
      const target = threadTarget(thread);
      addOption(targets, {
        target,
        kind: 'thread',
        label: thread.topic || thread.summary || thread.id,
        description: `${target} · ${t('sessionMessageCountLabel')} ${thread.message_count}`,
        source: 'thread',
      });
    }

    for (const runtime of runtimesQuery.data ?? []) {
      addOption(targets, {
        target: `dm:@${runtime.id}`,
        kind: 'dm',
        label: runtime.display_name || runtime.name || runtime.id,
        description: `dm:@${runtime.id}`,
        source: 'runtime',
        disabled: !runtime.enabled,
      });
    }

    return Array.from(targets.values()).sort((a, b) => {
      const order: Record<CollaborationTargetKind, number> = {
        channel: 0,
        thread: 1,
        dm: 2,
        route: 3,
        unknown: 4,
      };
      if (order[a.kind] !== order[b.kind]) {
        return order[a.kind] - order[b.kind];
      }
      return a.label.localeCompare(b.label);
    });
  }, [channelsQuery.data, runtimesQuery.data, threadsQuery.data]);

  const optionMap = useMemo(() => new Map(options.map((option) => [option.target, option])), [options]);
  const isLoading = channelsQuery.isLoading || threadsQuery.isLoading || runtimesQuery.isLoading;

  return { options, optionMap, isLoading };
}

export function describeCollaborationTarget(
  target: string,
  optionMap?: Map<string, CollaborationTargetOption>,
): CollaborationTargetOption {
  const normalized = normalizeTarget(target);
  const known = optionMap?.get(normalized);
  if (known) {
    return known;
  }
  return {
    target: normalized || target,
    kind: kindFromTarget(normalized || target),
    label: normalized || target || '-',
    description: normalized || target || '-',
    source: 'raw',
  };
}

function stringField(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

export function parseNotificationRouteTarget(route?: NotificationRoute | null): RouteTargetPreview | null {
  if (!route) {
    return null;
  }
  let parsed: Record<string, unknown> = {};
  try {
    const value = JSON.parse(route.target_config_json || '{}');
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      parsed = value as Record<string, unknown>;
    }
  } catch (error) {
    return {
      target: '',
      title: route.name,
      detail: route.target_config_json || '{}',
      config: {},
      parseError: error instanceof Error ? error.message : t('notificationRoutesInvalidTargetConfig'),
    };
  }

  const target = stringField(parsed.target);
  const chatID = stringField(parsed.chat_id);
  const userID = stringField(parsed.user_id);
  const sessionID = stringField(parsed.session_id);
  const replyTo = stringField(parsed.reply_to);
  const title = stringField(parsed.title) || route.name;
  const parts = [
    target ? `target=${target}` : '',
    chatID ? `chat=${chatID}` : '',
    userID ? `user=${userID}` : '',
    sessionID ? `session=${sessionID}` : '',
    replyTo ? `reply_to=${replyTo}` : '',
  ].filter(Boolean);

  return {
    target: target || chatID || userID || sessionID,
    title,
    detail: parts.length > 0 ? parts.join(' · ') : t('collaborationTargetRouteUnspecified'),
    config: parsed,
  };
}
