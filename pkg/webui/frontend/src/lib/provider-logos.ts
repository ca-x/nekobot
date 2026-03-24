type ProviderLogoMap = Record<string, string>;

const PROVIDER_LOGOS: ProviderLogoMap = {
  openai: 'openai.svg',
  anthropic: 'anthropic.svg',
  gemini: 'gemini.svg',
  openrouter: 'openrouter.svg',
  groq: 'groq.svg',
  vllm: 'vllm.svg',
  zhipu: 'zhipu.svg',
  deepseek: 'deepseek.png',
  moonshot: 'moonshot.png',
};

export function getProviderLogo(name: string): string | null {
  const key = name.trim().toLowerCase();
  const file = PROVIDER_LOGOS[key];
  return file ? `/logos/${file}` : null;
}
