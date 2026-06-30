import React, { useMemo, useState } from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ConfigProvider, App as AntApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import App from './App';
import { AuthProvider } from './context/AuthContext';
import {
  ThemeContext,
  applyThemeMode,
  getInitialThemeMode,
} from './context/ThemeContext';
import { createAppTheme, type AppThemeMode } from './theme';
import './styles/index.css';

const initialThemeMode = getInitialThemeMode();
applyThemeMode(initialThemeMode);

function Root() {
  const [mode, setModeState] = useState<AppThemeMode>(initialThemeMode);

  const setMode = (next: AppThemeMode) => {
    setModeState(next);
    applyThemeMode(next);
  };

  const toggleMode = () => {
    setMode(mode === 'dark' ? 'light' : 'dark');
  };

  const theme = useMemo(() => createAppTheme(mode), [mode]);
  const themeContext = useMemo(() => ({ mode, setMode, toggleMode }), [mode]);

  return (
    <ThemeContext.Provider value={themeContext}>
      <ConfigProvider locale={zhCN} theme={theme}>
        <AntApp>
          <BrowserRouter>
            <AuthProvider>
              <App />
            </AuthProvider>
          </BrowserRouter>
        </AntApp>
      </ConfigProvider>
    </ThemeContext.Provider>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
);
