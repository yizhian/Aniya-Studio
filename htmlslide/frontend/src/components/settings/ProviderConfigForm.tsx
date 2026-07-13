import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { Loader2 } from "lucide-react";
import { useSettings } from "../../hooks/useSettings";
import { useLocale } from "../../context/LocaleContext";
import { ModelNameField } from "./ModelNameField";
import { ResultBanner } from "../common/ResultBanner";

interface ProviderRadioProps {
  selected: boolean;
  label: string;
  onSelect: () => void;
}

function ProviderRadio({ selected, label, onSelect }: ProviderRadioProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={`flex-1 px-3 py-2 rounded-xl text-sm font-medium transition-all border ${
        selected
          ? "bg-[var(--editor-accent)] text-[var(--editor-accent-text)] border-[var(--editor-accent)]"
          : "bg-[var(--editor-control)]/40 text-[var(--editor-text-muted)] border-[var(--editor-border)] hover:text-[var(--editor-text)]"
      }`}
    >
      {label}
    </button>
  );
}

interface SettingInputProps {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
}

function SettingInput({ label, value, onChange, type = "text", placeholder }: SettingInputProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-[11px] font-semibold uppercase tracking-wider text-[var(--editor-text-muted)]">
        {label}
      </label>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full px-3 py-2 rounded-xl bg-[var(--editor-control)]/50 border border-[var(--editor-border)] text-sm text-[var(--editor-text)] placeholder:text-[var(--editor-text-muted)] focus:outline-none focus:border-[var(--editor-accent)]/50 transition-colors"
      />
    </div>
  );
}

export function ProviderConfigForm() {
  const { t } = useLocale();
  const {
    settings,
    updateSettings,
    updateProvider,
    testConnection,
    syncToAgent,
    fetchModels,
  } = useSettings();
  const [testing, setTesting] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [loadingModels, setLoadingModels] = useState(false);
  const [availableModels, setAvailableModels] = useState<string[]>([]);
  const [modelsHint, setModelsHint] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [syncResult, setSyncResult] = useState<{ ok: boolean; message: string } | null>(null);

  const provider = settings.selectedProvider;
  const cfg = settings.providers[provider];

  useEffect(() => {
    setAvailableModels([]);
    setModelsHint(null);
  }, [provider]);

  async function handleFetchModels() {
    if (!cfg.apiKey || !cfg.baseUrl) return;
    setLoadingModels(true);
    setModelsHint(null);
    const result = await fetchModels(provider);
    setAvailableModels(result.models);
    if (result.error) {
      setModelsHint(result.error);
    } else if (result.models.length === 0) {
      setModelsHint(t.settings.modelsUnavailable);
    } else {
      setModelsHint(t.settings.modelsFetched.replace("{n}", String(result.models.length)));
    }
    setLoadingModels(false);
  }

  async function handleTest() {
    setTesting(true);
    setTestResult(null);
    const result = await testConnection(provider);
    setTestResult(result);
    setTesting(false);
  }

  async function handleSave() {
    setSyncing(true);
    setSyncResult(null);
    const result = await syncToAgent(provider);
    setSyncResult(
      result.ok
        ? { ok: true, message: result.message || t.settings.syncSuccess }
        : { ok: false, message: result.message || t.settings.syncFailed },
    );
    setSyncing(false);
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <label className="text-[11px] font-semibold uppercase tracking-wider text-[var(--editor-text-muted)]">
          {t.settings.provider}
        </label>
        <div className="flex gap-2">
          <ProviderRadio
            selected={provider === "anthropic"}
            label="Anthropic"
            onSelect={() => updateSettings({ selectedProvider: "anthropic" })}
          />
          <ProviderRadio
            selected={provider === "openai"}
            label="OpenAI / Compatible"
            onSelect={() => updateSettings({ selectedProvider: "openai" })}
          />
        </div>
      </div>

      <div className="flex flex-col gap-4">
        <SettingInput
          label={t.settings.apiKey}
          value={cfg.apiKey}
          onChange={(v) => updateProvider(provider, { apiKey: v })}
          type="password"
          placeholder={provider === "anthropic" ? "sk-ant-..." : "sk-..."}
        />
        <SettingInput
          label={t.settings.baseUrl}
          value={cfg.baseUrl}
          onChange={(v) => updateProvider(provider, { baseUrl: v })}
          placeholder="https://api.anthropic.com"
        />
        <ModelNameField
          value={cfg.modelName}
          onChange={(v) => updateProvider(provider, { modelName: v })}
          models={availableModels}
          placeholder={provider === "anthropic" ? "claude-opus-4-5" : "gpt-4o"}
          hint={modelsHint}
          onFetch={handleFetchModels}
          fetching={loadingModels}
          fetchLabel={t.settings.fetchModels}
          loadingLabel={t.settings.loadingModels}
          pickLabel={t.settings.modelName}
          disabled={!cfg.apiKey || !cfg.baseUrl}
        />
      </div>

      <div className="flex gap-2">
        <button
          type="button"
          onClick={handleTest}
          disabled={testing || syncing || !cfg.apiKey || !cfg.modelName}
          className="flex flex-1 items-center justify-center gap-2 py-2.5 rounded-xl text-sm font-medium border transition-all disabled:opacity-40 disabled:cursor-not-allowed"
          style={{ borderColor: "var(--editor-text)", color: "var(--editor-text)" }}
        >
          {testing ? (
            <>
              <Loader2 size={14} className="animate-spin" />
              {t.settings.testing}
            </>
          ) : (
            t.settings.testConnection
          )}
        </button>

        <button
          type="button"
          onClick={handleSave}
          disabled={testing || syncing || !cfg.apiKey || !cfg.modelName}
          className="flex flex-1 items-center justify-center gap-2 py-2.5 rounded-xl text-sm font-semibold transition-all disabled:opacity-40 disabled:cursor-not-allowed"
          style={{
            backgroundColor: "var(--editor-text)",
            color: "var(--editor-bg)",
          }}
        >
          {syncing ? (
            <>
              <Loader2 size={14} className="animate-spin" />
              {t.settings.syncing}
            </>
          ) : (
            t.settings.saveAndSync
          )}
        </button>
      </div>

      <div className="flex flex-col gap-2">
        <ResultBanner result={testResult} />
        <ResultBanner result={syncResult} />
      </div>

      <p className="text-[11px] text-[var(--editor-text-muted)] leading-relaxed">
        {t.settings.modelsHint}
      </p>
    </div>
  );
}
