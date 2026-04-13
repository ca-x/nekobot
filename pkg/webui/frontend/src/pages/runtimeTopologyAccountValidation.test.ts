import test from 'node:test';
import assert from 'node:assert/strict';

import { getChannelAccountValidationMessage } from './runtimeTopologyAccountValidation.ts';

test('enabled wechat accounts require bot token and iLink bot id', () => {
  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'wechat',
      enabled: true,
      config: {},
      t: (key: string) => key,
    }),
    'runtimeTopologyWechatCredentialsHint',
  );

  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'wechat',
      enabled: true,
      config: { bot_token: 'token-only' },
      t: (key: string) => key,
    }),
    'runtimeTopologyWechatCredentialsHint',
  );

  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'wechat',
      enabled: true,
      config: { bot_token: 'token', ilink_bot_id: 'bot@im.wechat' },
      t: (key: string) => key,
    }),
    null,
  );
});

test('enabled gotify accounts require server url and app token', () => {
  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'gotify',
      enabled: true,
      config: {},
      t: (key: string) => key,
    }),
    'runtimeTopologyGotifyCredentialsHint',
  );

  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'gotify',
      enabled: true,
      config: { server_url: 'https://gotify.example.com' },
      t: (key: string) => key,
    }),
    'runtimeTopologyGotifyCredentialsHint',
  );

  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'gotify',
      enabled: true,
      config: { server_url: 'https://gotify.example.com', app_token: 'token' },
      t: (key: string) => key,
    }),
    null,
  );
});

test('disabled wechat/gotify accounts and unrelated channel types stay valid without credentials', () => {
  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'wechat',
      enabled: false,
      config: {},
      t: (key: string) => key,
    }),
    null,
  );

  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'gotify',
      enabled: false,
      config: {},
      t: (key: string) => key,
    }),
    null,
  );

  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'discord',
      enabled: true,
      config: {},
      t: (key: string) => key,
    }),
    null,
  );
});

test('invalid account config JSON is treated as invalid before submit', () => {
  assert.equal(
    getChannelAccountValidationMessage({
      channelType: 'wechat',
      enabled: true,
      config: null as unknown as Record<string, unknown>,
      t: (key: string) => key,
    }),
    'runtimeTopologyInvalidJsonHint',
  );
});
