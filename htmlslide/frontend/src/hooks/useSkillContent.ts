import { useEffect, useState } from "react";
import { fetchSkillContent } from "../api/skills";
import { extractErrorMessage } from "../models/apiResponses";

interface SkillContentState {
  content: string | null;
  loading: boolean;
  error: string | null;
}

export function useSkillContent(skillName: string | null) {
  const [state, setState] = useState<SkillContentState>({
    content: null,
    loading: false,
    error: null,
  });

  useEffect(() => {
    if (!skillName) {
      setState({ content: null, loading: false, error: null });
      return;
    }

    let cancelled = false;
    setState({ content: null, loading: true, error: null });

    fetchSkillContent(skillName)
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(await extractErrorMessage(res));
        }
        return res.json();
      })
      .then((data) => {
        if (!cancelled) {
          setState({
            content: data.content ?? "",
            loading: false,
            error: null,
          });
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setState({
            content: null,
            loading: false,
            error: err instanceof Error ? err.message : "Failed to load",
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [skillName]);

  return state;
}
