export type View = "dashboard" | "users" | "myKeys" | "providers" | "models";

export type Session = { user: AppUser };
export type AppUser = { id: number; username: string; role: string; valid: boolean };
export type UserAPIKey = { id: number; user_id: number; name: string; token: string; valid: boolean };

export type ProviderRecord = {
  id: number;
  name: string;
  type: string;
  preset_id: string;
  base_url: string;
  enabled: boolean;
};

export type ProviderPreset = {
  id: string;
  name: string;
  type: string;
  base_url: string;
  description: string;
};

export type ProviderKey = {
  id: number;
  provider_id: number;
  provider_name?: string;
  name: string;
  secret: string;
  secret_hint: string;
  enabled: boolean;
  priority: number;
  weight: number;
  labels?: string;
  disabled_status?: number;
  disabled_error_code?: string;
  disabled_error_message?: string;
  disabled_error_body?: string;
  disabled_at?: string;
};

export type Model = {
  id: number;
  name: string;
  display_name: string;
  description: string;
  context_window: number;
  max_input_tokens: number;
  max_output_tokens: number;
  capabilities: string;
  selection_policy: string;
  enabled: boolean;
};

export type ModelPreset = {
  id: string;
  family: string;
  name: string;
  display_name: string;
  description: string;
  context_window: number;
  max_input_tokens: number;
  max_output_tokens: number;
  capabilities: string;
  selection_policy: string;
};

export type ModelRoute = {
  id: number;
  model_id: number;
  provider_id: number;
  provider_name?: string;
  provider_type?: string;
  upstream_model: string;
  quota_family: string;
  enabled: boolean;
  priority: number;
  weight: number;
};

export type KeyError = { key: string; model: string; status: number; message: string; updated_at: string };

export type AdminConfig = {
  addr: string;
  upstream_base_url: string;
  timeout: string;
  max_attempts: number;
  cooldown_seconds: number;
};

export type AdminData = {
  config: AdminConfig;
  users: AppUser[];
  user_api_keys: UserAPIKey[];
};

export type Stats = {
  started_at: string;
  requests: number;
  by_key: Record<string, number>;
  exhausted: Record<string, string>;
  rpd_like: Record<string, boolean>;
  key_errors: Record<string, KeyError>;
  last_updated: string;
};

export type UsageSummary = {
  requests: number;
  errors: number;
  avg_latency_ms: number;
  by_key: Record<string, number>;
  series: UsagePoint[];
  bucket_minutes: number;
};

export type UsagePoint = {
  date: string;
  requests: number;
  errors: number;
  avg_latency_ms: number;
};
