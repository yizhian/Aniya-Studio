import {
  createContext,
  useCallback,
  useContext,
  type ReactNode,
} from "react";
import { useLocalStorage } from "./useLocalStorage";

export type ProviderKey = "anthropic" | "openai";

export interface ProviderConfig {
  apiKey: string;
  baseUrl: string;
  modelName: string;
}

export interface AppSettings {
  selectedProvider: ProviderKey;
  providers: Record<ProviderKey, ProviderConfig>;
}

const STORAGE_KEY = "aniya-studio-settings";

const DEFAULT_PROVIDERS: Record<ProviderKey, ProviderConfig> = {
  anthropic: {
    apiKey: "",
    baseUrl: "https://api.anthropic.com",
    modelName: "claude-opus-4-5",
  },
  openai: {
    apiKey: "",
    baseUrl: "https://api.openai.com",
    modelName: "gpt-4o",
  },
};

export interface TestResult {
  ok: boolean;
  message: string;
  inList?: boolean;
  verified?: boolean;
}

export interface ModelsResult {
  models: string[];
  source: string;
  error?: string;
}

export function providerPayload(provider: ProviderKey, cfg: ProviderConfig) {
  return {
    provider,
    api_key: cfg.apiKey,
    base_url: cfg.baseUrl,
    model_name: cfg.modelName,
  };
}

interface SettingsContextValue {
  settings: AppSettings;
  updateSettings: (updates: Partial<AppSettings>) => void;
  updateProvider: (key: ProviderKey, patch: Partial<ProviderConfig>) => void;
  testConnection: (provider?: ProviderKey) => Promise<TestResult>;
  syncToAgent: (provider?: ProviderKey) => Promise<TestResult>;
  fetchModels: (provider?: ProviderKey) => Promise<ModelsResult>;
  getActiveConfig: () => { provider: ProviderKey; cfg: ProviderConfig };
}

const SettingsCtx = createContext<SettingsContextValue | null>(null);

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [settings, setSettingsState] = useLocalStorage<AppSettings>(
    STORAGE_KEY,
    { selectedProvider: "anthropic", providers: DEFAULT_PROVIDERS },
  );

  const updateSettings = useCallback((updates: Partial<AppSettings>) => {
    setSettingsState((prev) => {
      const next = { ...prev, ...updates };
      return next;
    });
  }, [setSettingsState]);

  const updateProvider = useCallback((key: ProviderKey, patch: Partial<ProviderConfig>) => {
    setSettingsState((prev) => {
      const next: AppSettings = {
        ...prev,
        providers: {
          ...prev.providers,
          [key]: { ...prev.providers[key], ...patch },
        },
      };
      return next;
    });
  }, [setSettingsState]);

  const getActiveConfig = useCallback(() => {
    const provider = settings.selectedProvider;
    return { provider, cfg: settings.providers[provider] };
  }, [settings]);

  const testConnection = useCallback(
    async (providerKey?: ProviderKey): Promise<TestResult> => {
      const provider = providerKey ?? settings.selectedProvider;
      const cfg = settings.providers[provider];
      try {
        const res = await fetch("/api/v1/settings/test-connection", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(providerPayload(provider, cfg)),
        });
        const data = await res.json();
        return {
          ok: data.ok ?? false,
          message: data.message ?? "Unknown response",
          inList: data.in_list,
          verified: data.verified,
        };
      } catch (err) {
        return {
          ok: false,
          message: err instanceof Error ? err.message : "Request failed",
        };
      }
    },
    [settings],
  );

  const syncToAgent = useCallback(
    async (providerKey?: ProviderKey): Promise<TestResult> => {
      const provider = providerKey ?? settings.selectedProvider;
      const cfg = settings.providers[provider];
      try {
        const res = await fetch("/api/v1/settings/sync", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(providerPayload(provider, cfg)),
        });
        const data = await res.json();
        return { ok: data.ok ?? false, message: data.message ?? "Unknown response" };
      } catch (err) {
        return {
          ok: false,
          message: err instanceof Error ? err.message : "Request failed",
        };
      }
    },
    [settings],
  );

  const fetchModels = useCallback(
    async (providerKey?: ProviderKey): Promise<ModelsResult> => {
      const provider = providerKey ?? settings.selectedProvider;
      const cfg = settings.providers[provider];
      try {
        const res = await fetch("/api/v1/settings/models", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(providerPayload(provider, cfg)),
        });
        const data = await res.json();
        return {
          models: data.models ?? [],
          source: data.source ?? "none",
          error: data.error,
        };
      } catch (err) {
        return {
          models: [],
          source: "none",
          error: err instanceof Error ? err.message : "Request failed",
        };
      }
    },
    [settings],
  );

  return (
    <SettingsCtx.Provider
      value={{
        settings,
        updateSettings,
        updateProvider,
        testConnection,
        syncToAgent,
        fetchModels,
        getActiveConfig,
      }}
    >
      {children}
    </SettingsCtx.Provider>
  );
}

export function useSettings(): SettingsContextValue {
  const ctx = useContext(SettingsCtx);
  if (!ctx) throw new Error("useSettings must be used within SettingsProvider");
  return ctx;
}
