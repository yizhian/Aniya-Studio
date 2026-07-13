import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { SkillTemplatePicker } from "../SkillTemplatePicker";

// IntersectionObserver is not available in jsdom
class MockIntersectionObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  constructor(_callback: IntersectionObserverCallback, _options?: IntersectionObserverInit) {}
}
beforeEach(() => {
  vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);
});

// Mock useSkillList hook
const mockSkills = [
  {
    name: "modern-clean",
    description: "A modern clean design",
    triggers: "deck",
    scenario: "marketing",
    has_assets: true,
    has_preview: true,
  },
  {
    name: "dark-theme",
    description: "Dark theme for presentations",
    triggers: "deck",
    scenario: "pitch-deck",
    has_assets: false,
    has_preview: false,
  },
];

vi.mock("../../../hooks/useSkillList", () => ({
  useSkillList: () => ({
    skills: mockSkills,
    loading: false,
    error: null,
  }),
}));

// Mock SkillPreviewPanel to avoid complex iframe rendering
vi.mock("../../chat/SkillPreviewPanel", () => ({
  SkillPreviewPanel: ({ onClose }: { skill: unknown; onClose: () => void }) => (
    <div data-testid="preview-panel">
      <button onClick={onClose}>Close Preview</button>
    </div>
  ),
}));

describe("SkillTemplatePicker", () => {
  const defaultProps = {
    selectedSkill: null as string | null,
    onSelect: vi.fn(),
    onClear: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders skill templates button when no skill selected", () => {
    render(<SkillTemplatePicker {...defaultProps} />);
    // The button uses title attribute for the label text
    expect(screen.getByTitle("home.skillTemplates")).toBeDefined();
  });

  it("shows selected skill name with clear button when skill is selected", () => {
    render(
      <SkillTemplatePicker {...defaultProps} selectedSkill="modern-clean" />
    );
    expect(screen.getByText("modern-clean")).toBeDefined();
    const clearButton = screen.getByTitle("home.clearSkillTemplate");
    fireEvent.click(clearButton);
    expect(defaultProps.onClear).toHaveBeenCalledTimes(1);
  });

  it("opens gallery on button click", async () => {
    render(<SkillTemplatePicker {...defaultProps} />);

    const triggerButton = screen.getByTitle("home.skillTemplates");
    await act(async () => {
      fireEvent.click(triggerButton);
    });

    await screen.findByText("modern-clean");
    await screen.findByText("dark-theme");
  });

  it("displays skill names and descriptions in gallery", async () => {
    render(<SkillTemplatePicker {...defaultProps} />);

    const triggerButton = screen.getByTitle("home.skillTemplates");
    await act(async () => {
      fireEvent.click(triggerButton);
    });

    expect(screen.getByText("modern-clean")).toBeDefined();
    expect(screen.getByText("A modern clean design")).toBeDefined();
    expect(screen.getByText("dark-theme")).toBeDefined();
  });

  it("calls onSelect when 'Use this style' is clicked", async () => {
    render(<SkillTemplatePicker {...defaultProps} />);

    const triggerButton = screen.getByTitle("home.skillTemplates");
    await act(async () => {
      fireEvent.click(triggerButton);
    });

    const useButtons = screen.getAllByText("home.useSkillTemplate");
    await act(async () => {
      fireEvent.click(useButtons[0]);
    });

    expect(defaultProps.onSelect).toHaveBeenCalledWith(
      "modern-clean",
      expect.any(String)
    );
  });

  it("opens preview when preview button is clicked", async () => {
    render(<SkillTemplatePicker {...defaultProps} />);

    const triggerButton = screen.getByTitle("home.skillTemplates");
    await act(async () => {
      fireEvent.click(triggerButton);
    });

    const previewButtons = screen.getAllByText("home.skillPreview");
    await act(async () => {
      fireEvent.click(previewButtons[0]);
    });

    expect(screen.getByTestId("preview-panel")).toBeDefined();
  });
});
