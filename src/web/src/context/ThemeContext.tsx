import { createContext, useContext } from 'react';
import type { AppThemeMode } from '../theme';

export const THEME_MODE_KEY = 'polyglot_theme_mode_v2';

interface ThemeContextValue {
  mode: AppThemeMode;
  setMode: (mode: AppThemeMode) => void;
  toggleMode: () => void;
}

export const ThemeContext = createContext<ThemeContextValue | undefined>(undefined);

export function getInitialThemeMode(): AppThemeMode {
  const saved = localStorage.getItem(THEME_MODE_KEY);
  if (saved === 'light' || saved === 'dark') return saved;
  return 'light';
}

export function applyThemeMode(mode: AppThemeMode) {
  document.documentElement.dataset.theme = mode;
  localStorage.setItem(THEME_MODE_KEY, mode);
}

export function useThemeMode() {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useThemeMode must be used within ThemeContext.Provider');
  }
  return ctx;
}
