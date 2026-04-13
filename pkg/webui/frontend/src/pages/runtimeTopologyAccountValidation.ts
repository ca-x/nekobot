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

  const normalizedChannelType = channelType.trim().toLowerCase();
  if (normalizedChannelType === 'gotify') {
    const serverURL = typeof config.server_url === 'string' ? config.server_url.trim() : '';
    const appToken = typeof config.app_token === 'string' ? config.app_token.trim() : '';
    if (serverURL !== '' && appToken !== '') {
      return null;
    }
    return t('runtimeTopologyGotifyCredentialsHint');
  }

  if (normalizedChannelType !== 'wechat') {
    return null;
  }

  const botToken = typeof config.bot_token === 'string' ? config.bot_token.trim() : '';
  const iLinkBotID = typeof config.ilink_bot_id === 'string' ? config.ilink_bot_id.trim() : '';
  if (botToken !== '' && iLinkBotID !== '') {
    return null;
  }

  return t('runtimeTopologyWechatCredentialsHint');
}
