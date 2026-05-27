import type { ReactNode } from "react";

export type View = "dashboard" | "users" | "myKeys" | "accounts" | "runtime";

export type Session = { user: AppUser };
export type AppUser = { id: number; username: string; role: string; valid: boolean };
export type UserAPIKey = { id: number; user_id: number; name: string; token: string; valid: boolean };
export type GoogleAccount = { id: number; email: string; prefix: string; enabled: boolean };

export type AdminKey = {
  id: number;
  account_id: number;
  name: string;
  key: string;
  enabled: boolean;
  account_email: string;
  account_prefix: string;
};

export type KeyError = { key: string; model: string; status: number; message: string; updated_at: string };

export type AdminConfig = {
  addr: string;
  upstream_base_url: string;
  timeout: string;
  max_attempts: number;
  cooldown_seconds: number;
  models: string[];
};

export type AdminData = {
  config: AdminConfig;
  users: AppUser[];
  user_api_keys: UserAPIKey[];
  accounts: GoogleAccount[];
  keys: AdminKey[];
};

export type Stats = {
  started_at: string;
  requests: number;
  by_key: Record<string, number>;
  exhausted: Record<string, string>;
  key_errors: Record<string, KeyError>;
  last_updated: string;
};

export type UsagePoint = { date: string; requests: number; errors: number; avg_latency_ms: number };

export type UsageSummary = {
  requests: number;
  errors: number;
  avg_latency_ms: number;
  by_key: Record<string, number>;
  series: UsagePoint[];
};

export type MetricValue = ReactNode;
