import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { SkillsTab } from "../SkillsTab";

const mockSkills = [
  { name: "skill-a", description: "Desc A", triggers: ["trigger1"], scenario: "marketing" },
  { name: "skill-b", description: "Desc B", triggers: [], scenario: "pitch-deck" },
];

vi.mock("../../../context/LocaleContext", () => ({
  useLocale: () => ({
    t: {
      settings: {
        scenario: "Scenario",
        skillsCount: "Found {n} skills",
      },
      home: { noSkillsAvailable: "No skills" },
    },
  }),
}));

vi.mock("../../../hooks/useSkillList", () => ({
  useSkillList: vi.fn(),
}));

vi.mock("../../../hooks/useSkillContent", () => ({
  useSkillContent: () => ({ content: null, loading: false, error: null }),
}));

import { useSkillList } from "../../../hooks/useSkillList";

describe("SkillsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading spinner when loading and no skills", () => {
    vi.mocked(useSkillList).mockReturnValue({ skills: [], loading: true, error: null });
    render(<SkillsTab />);
    const spinner = document.querySelector(".animate-spin");
    expect(spinner).toBeTruthy();
  });

  it("shows error message on error", () => {
    vi.mocked(useSkillList).mockReturnValue({ skills: [], loading: false, error: "Fetch failed" });
    render(<SkillsTab />);
    expect(screen.getByText("Fetch failed")).toBeTruthy();
  });

  it("shows empty state when no skills", () => {
    vi.mocked(useSkillList).mockReturnValue({ skills: [], loading: false, error: null });
    render(<SkillsTab />);
    expect(screen.getByText("No skills")).toBeTruthy();
  });

  it("renders skill count and skill names", () => {
    vi.mocked(useSkillList).mockReturnValue({ skills: mockSkills, loading: false, error: null });
    render(<SkillsTab />);
    expect(screen.getByText("Found 2 skills")).toBeTruthy();
    expect(screen.getByText("skill-a")).toBeTruthy();
    expect(screen.getByText("skill-b")).toBeTruthy();
  });

  it("shows skill description when collapsed", () => {
    vi.mocked(useSkillList).mockReturnValue({ skills: mockSkills, loading: false, error: null });
    render(<SkillsTab />);
    expect(screen.getByText("Desc A")).toBeTruthy();
  });
});
