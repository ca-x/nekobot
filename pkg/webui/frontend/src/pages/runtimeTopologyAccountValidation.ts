type TranslateFn = (key: string) => string;

type ChannelAccountValidationInput = {
  channelType: string;
  enabled: boolean;
  config: Record<string, unknown> | null;
  t: TranslateFn;
};

export function getChannelAccountValidationMessage({
  channelType,
  enabled,
  config,
  t,
}: ChannelAccountValidationInput): string | null {
  if (!config || Array.isArray(config) || typeof config !== 'object') {
    return t('runtimeTopologyInvalidJsonHint');
  }

  if (!enabled) {
    return null;
  }

  if (channelType.trim().toLowerCase() !== 'wechat') {
    return null;
  }

  const botToken = typeof config.bot_token === 'string' ? config.bot_token.trim() : '';
  const iLinkBotID = typeof config.ilink_bot_id === 'string' ? config.ilink_bot_id.trim() : '';
  if (botToken !== '' && iLinkBotID !== '') {
    return null;
  }

  return t('runtimeTopologyWechatCredentialsHint');
}
