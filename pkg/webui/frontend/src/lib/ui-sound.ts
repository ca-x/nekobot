import { error as playErrorTone, init, notify as playNotifyTone, setVolume, success as playSuccessTone, toggle as playToggleTone } from '@rexa-developer/tiks';

const DEFAULT_VOLUME = 0.24;
let initialized = false;

function ensureInit() {
  if (initialized || typeof window === 'undefined') {
    return;
  }
  try {
    init();
    setVolume(DEFAULT_VOLUME);
    initialized = true;
  } catch {
    // Keep UI resilient when Web Audio is unavailable or blocked.
  }
}

function runIfEnabled(enabled: boolean, fn: () => void) {
  if (!enabled) {
    return;
  }
  ensureInit();
  try {
    fn();
  } catch {
    // Never let UI sounds break interaction flows.
  }
}

export function playSuccessSound(enabled: boolean) {
  runIfEnabled(enabled, () => playSuccessTone());
}

export function playErrorSound(enabled: boolean) {
  runIfEnabled(enabled, () => playErrorTone());
}

export function playNotifySound(enabled: boolean) {
  runIfEnabled(enabled, () => playNotifyTone());
}

export function playToggleSound(enabled: boolean, checked = true) {
  runIfEnabled(enabled, () => playToggleTone(checked));
}
