import { useEffect, useState } from "react";
import { fetchSkillList } from "../api/skills";

export interface SkillItem {
  name: string;
  description: string;
  triggers?: string[];
  scenario?: string;
  has_assets?: boolean;
  has_preview?: boolean;
}

interface SkillListState {
  skills: SkillItem[];
  loading: boolean;
  error: string | null;
}

export function useSkillList(mode = "deck") {
  const [state, setState] = useState<SkillListState>({
    skills: [],
    loading: false,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;
    setState((s) => ({ ...s, loading: true, error: null }));

    fetchSkillList(mode)
      .then((res) => res.json())
      .then((data) => {
        if (!cancelled) {
          setState({ skills: data.skills ?? [], loading: false, error: null });
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setState({ skills: [], loading: false, error: err.message });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [mode]);

  return state;
}
