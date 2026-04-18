import { toast as sonnerToast, type ExternalToast } from 'sonner';
import { getUiSoundEnabled } from '@/stores/ui.store';
import { playErrorSound, playNotifySound, playSuccessSound } from '@/lib/ui-sound';

function soundEnabled() {
  return getUiSoundEnabled();
}

export const toast = {
  ...sonnerToast,
  success(message: string, data?: ExternalToast) {
    playSuccessSound(soundEnabled());
    return sonnerToast.success(message, data);
  },
  error(message: string, data?: ExternalToast) {
    playErrorSound(soundEnabled());
    return sonnerToast.error(message, data);
  },
  info(message: string, data?: ExternalToast) {
    playNotifySound(soundEnabled());
    return sonnerToast.info(message, data);
  },
  warning(message: string, data?: ExternalToast) {
    playNotifySound(soundEnabled());
    return sonnerToast.warning(message, data);
  },
  message(message: string, data?: ExternalToast) {
    playNotifySound(soundEnabled());
    return sonnerToast.message(message, data);
  },
};
