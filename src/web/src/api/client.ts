import axios from 'axios';
import type { AxiosInstance } from 'axios';
import type {
  User,
  Role,
  UserRole,
  APIKey,
  Provider,
  Proxy,
  ProviderProxyView,
  ModelMapping,
  Adapter,
  AdapterInstance,
  RequestLog,
  UsageEvent,
  Stats,
  RequestLogFilter,
  UsageEventFilter,
  LoginResponse,
  AuthUser,
} from './types';

export const TOKEN_KEY = 'polyglot_token';

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || '';
}

export function setToken(token: string) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token);
  } else {
    localStorage.removeItem(TOKEN_KEY);
  }
}

const client = axios.create({
  baseURL: '/api/admin',
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
});

function installAuthInterceptors(instance: AxiosInstance) {
  instance.interceptors.request.use((config) => {
    const token = getToken();
    if (token) {
      config.headers = config.headers || {};
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  });

  instance.interceptors.response.use(
    (resp) => resp,
    (error) => {
      if (error?.response?.status === 401) {
        setToken('');
        if (!window.location.pathname.endsWith('/login')) {
          window.location.assign('/login');
        }
      }
      return Promise.reject(error);
    },
  );
}

installAuthInterceptors(client);

// 去掉 undefined/空串的查询参数
function params(obj: object) {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(obj)) {
    if (v !== undefined && v !== null && v !== '') out[k] = v;
  }
  return out;
}

export default client;

// ---- Auth ----
export const login = (email: string, password: string) =>
  client.post<LoginResponse>('/login', { email, password }).then((r) => r.data);

export const fetchProfile = () => client.get<AuthUser>('/profile').then((r) => r.data);

export const logout = () => client.post('/logout').then((r) => r.data);

export interface BootstrapInput {
  email: string;
  password: string;
  display_name?: string;
}
export const bootstrap = (input: BootstrapInput) =>
  client.post<LoginResponse>('/bootstrap', input).then((r) => r.data);

// ---- Stats ----
export const getStats = () => client.get<Stats>('/stats').then((r) => r.data);

// ---- Users ----
export const listUsers = () => client.get<User[]>('/users').then((r) => r.data);
export const upsertUser = (user: Partial<User>) =>
  client.post<User>('/users', user).then((r) => r.data);

// ---- Roles ----
export const listRoles = () => client.get<Role[]>('/roles').then((r) => r.data);
export const upsertRole = (role: Partial<Role>) =>
  client.post<Role>('/roles', role).then((r) => r.data);

// ---- User-Roles ----
export const listUserRoles = (userId: string) =>
  client.get<Role[]>('/user-roles', { params: { user_id: userId } }).then((r) => r.data);
export const assignRole = (ur: UserRole) =>
  client.post<UserRole>('/user-roles', ur).then((r) => r.data);

// ---- API Keys ----
export const listApiKeys = () => client.get<APIKey[]>('/api-keys').then((r) => r.data);
export const upsertApiKey = (key: Partial<APIKey>) =>
  client.post<APIKey>('/api-keys', key).then((r) => r.data);

// ---- Providers ----
export const listProviders = () => client.get<Provider[]>('/providers').then((r) => r.data);
export const upsertProvider = (provider: Partial<Provider>) =>
  client.post<Provider>('/providers', provider).then((r) => r.data);

export interface ProviderHealth {
  requests_total: number;
  success_rate: number; // 0..1
  avg_latency_ms: number;
}
// 24h per-provider health, keyed by provider name.
export const fetchProviderHealth = () =>
  client.get<Record<string, ProviderHealth>>('/providers/health').then((r) => r.data);

export interface HealthBucket {
  total: number;
  successes: number;
}
// 24-hourly health, slot 0 = oldest (23h ago), slot 23 = current hour.
export const fetchProviderHealthHourly = () =>
  client.get<Record<string, HealthBucket[]>>('/providers/health/hourly').then((r) => r.data);

// ---- Proxies (network egress, M:N with providers) ----
export const listProxies = () => client.get<Proxy[]>('/proxies').then((r) => r.data);
export const upsertProxy = (proxy: Partial<Proxy>) =>
  client.post<Proxy>('/proxies', proxy).then((r) => r.data);
export const deleteProxy = (id: string) =>
  client.delete<{ deleted: string }>(`/proxies/${id}`).then((r) => r.data);
export const testProxy = (id: string, target?: string) =>
  client
    .post<{
      success: boolean;
      proxy?: string;
      target?: string;
      status?: number;
      latency_ms?: number;
      exit_ip?: string;
      error?: string;
    }>(`/proxies/${id}/test`, { target })
    .then((r) => r.data);

// ---- Provider↔Proxy associations ----
export const listProviderProxies = (providerId: string) =>
  client.get<ProviderProxyView[]>(`/providers/${providerId}/proxies`).then((r) => r.data);
export const setProviderProxies = (
  providerId: string,
  assocs: { proxy_id: string; priority: number }[],
) => client.post(`/providers/${providerId}/proxies`, assocs).then((r) => r.data);

// ---- Model Mappings (provider-owned 1:N) ----
export const listProviderModelMappings = (providerId: string) =>
  client.get<ModelMapping[]>(`/providers/${providerId}/model-mappings`).then((r) => r.data);
export const upsertProviderModelMapping = (providerId: string, m: Partial<ModelMapping>) =>
  client.post<ModelMapping>(`/providers/${providerId}/model-mappings`, m).then((r) => r.data);
export const deleteModelMapping = (id: string) =>
  client.delete<{ deleted: string }>(`/model-mappings/${id}`).then((r) => r.data);

// ---- Adapters ----
export const listAdapters = () => client.get<Adapter[]>('/adapters').then((r) => r.data);
export const listAdapterInstances = (adapterId?: string) =>
  client
    .get<AdapterInstance[]>('/adapter-instances', { params: params({ adapter_id: adapterId }) })
    .then((r) => r.data);

// ---- Audit ----
export const listRequestLogs = (filter: RequestLogFilter = {}) =>
  client.get<RequestLog[]>('/request-logs', { params: params(filter) }).then((r) => r.data);
export const listUsageEvents = (filter: UsageEventFilter = {}) =>
  client.get<UsageEvent[]>('/usage-events', { params: params(filter) }).then((r) => r.data);
