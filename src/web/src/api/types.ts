// API 类型定义 —— 严格对齐后端 internal/domain/entities.go 的 JSON tag。
// 注意:多数实体是 snake_case;AdapterInstance / Account 等无 json tag,
// 会被 Go 序列化成 PascalCase(见 AdapterInstance)。

export type Status = 'active' | 'disabled' | 'healthy' | string;

export interface User {
  id: string;
  email: string;
  display_name: string;
  status: Status;
  last_login_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface Role {
  id: string;
  name: string;
  description: string;
  permissions: string;
  created_at: string;
  updated_at: string;
}

export interface UserRole {
  user_id: string;
  role_id: string;
}

export interface APIKey {
  id: string;
  user_id?: string;
  name: string;
  // 仅在创建/未脱敏时返回明文 key
  key?: string;
  scopes: string;
  status: Status;
  expires_at?: string | null;
  last_used_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface Provider {
  id: string;
  name: string;
  type: string;
  base_url: string;
  auth_type: string;
  default_headers: string;
  status: Status;
  created_at: string;
  updated_at: string;
}

export interface ModelMapping {
  id: string;
  provider_id: string;
  from_model: string;
  to_model: string;
  created_at: string;
  updated_at: string;
}

export interface Adapter {
  id: string;
  name: string;
  type: string;
  status: Status;
  created_at: string;
  updated_at: string;
}

// AdapterInstance 后端无 json tag → PascalCase 字段
export interface AdapterInstance {
  ID: string;
  AdapterID: string;
  Provider: string;
  CallbackAddr: string;
  Capabilities: string;
  Metadata: string;
  Status: string;
  LastHeartbeatAt?: string | null;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface RequestLog {
  id: string;
  user_id: string;
  api_key_id: string;
  provider: string;
  adapter_id: string;
  protocol: string;
  model: string;
  status_code: number;
  success: boolean;
  latency_ms: number;
  input_tokens: number;
  output_tokens: number;
  error_type: string;
  error_message: string;
  created_at: string;
}

export interface UsageEvent {
  id: string;
  user_id: string;
  account_id: string;
  provider: string;
  model: string;
  tokens_used: number;
  requests_count: number;
  created_at: string;
}

export interface Stats {
  requests_total: number;
  success_rate: number;
  average_latency_ms: number;
}

// 登录态当前用户(后端 /login、/profile 返回)
export interface AuthUser {
  id: string;
  email: string;
  display_name: string;
  status: Status;
}

export interface LoginResponse {
  token: string;
  user: AuthUser;
}

export interface RequestLogFilter {
  user_id?: string;
  provider?: string;
  protocol?: string;
  from?: string;
  to?: string;
  limit?: number;
}

export interface UsageEventFilter {
  user_id?: string;
  account_id?: string;
  provider?: string;
  model?: string;
  from?: string;
  to?: string;
  limit?: number;
}
