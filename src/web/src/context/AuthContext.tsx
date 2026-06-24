import { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import {
  getToken,
  setToken,
  login as apiLogin,
  fetchProfile,
  logout as apiLogout,
  bootstrap as apiBootstrap,
  type BootstrapInput,
} from '../api/client';
import type { AuthUser } from '../api/types';

interface AuthState {
  user: AuthUser | undefined;
  isAuthenticated: boolean;
  ready: boolean; // 会话恢复是否完成
  login: (email: string, password: string) => Promise<void>;
  bootstrap: (input: BootstrapInput) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser>();
  const [ready, setReady] = useState(false);

  // 刷新页面时:有 token 就用 /profile 恢复当前用户
  useEffect(() => {
    if (!getToken()) {
      setReady(true);
      return;
    }
    fetchProfile()
      .then(setUser)
      .catch(() => {
        setToken('');
        setUser(undefined);
      })
      .finally(() => setReady(true));
  }, []);

  const login = async (email: string, password: string) => {
    const resp = await apiLogin(email, password);
    setToken(resp.token);
    setUser(resp.user);
  };

  const bootstrap = async (input: BootstrapInput) => {
    const resp = await apiBootstrap(input);
    setToken(resp.token);
    setUser(resp.user);
  };

  const logout = () => {
    if (getToken()) {
      apiLogout().catch(() => undefined);
    }
    setToken('');
    setUser(undefined);
  };

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, ready, login, bootstrap, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
