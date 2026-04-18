import { create } from 'zustand';

const UI_SOUND_STORAGE_KEY = 'nekobot_ui_sound_enabled';

function loadUiSoundEnabled() {
  if (typeof window === 'undefined') {
    return false;
  }
  return window.localStorage.getItem(UI_SOUND_STORAGE_KEY) === 'true';
}

function persistUiSoundEnabled(enabled: boolean) {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(UI_SOUND_STORAGE_KEY, String(enabled));
}

interface UiState {
  sidebarOpen: boolean;
  mobileSidebarOpen: boolean;
  uiSoundEnabled: boolean;
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;
  toggleMobileSidebar: () => void;
  setMobileSidebarOpen: (open: boolean) => void;
  setUiSoundEnabled: (enabled: boolean) => void;
}

export const useUiStore = create<UiState>((set) => ({
  sidebarOpen: true,
  mobileSidebarOpen: false,
  uiSoundEnabled: loadUiSoundEnabled(),
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  setSidebarOpen: (open) => set({ sidebarOpen: open }),
  toggleMobileSidebar: () => set((s) => ({ mobileSidebarOpen: !s.mobileSidebarOpen })),
  setMobileSidebarOpen: (open) => set({ mobileSidebarOpen: open }),
  setUiSoundEnabled: (enabled) => {
    persistUiSoundEnabled(enabled);
    set({ uiSoundEnabled: enabled });
  },
}));

export function getUiSoundEnabled() {
  return loadUiSoundEnabled();
}
