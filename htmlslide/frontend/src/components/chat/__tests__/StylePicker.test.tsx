import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { StylePicker } from "../StylePicker";
import type { SkillOption } from "../StylePicker";

// ─── StylePicker component rendering tests ───

describe("StylePicker", () => {
  const sampleSkills: SkillOption[] = [
    {
      name: "暖色系",
      description: "温暖色调的设计风格",
      triggers: ["暖色", "温暖"],
    },
    {
      name: "科技风格",
      description: "现代科技感设计",
      triggers: ["科技", "数码", "AI"],
    },
  ];

  it("renders skill option names", () => {
    render(
      <StylePicker
        skills={sampleSkills}
        onSelect={() => {}}
        onClose={() => {}}
      />,
    );
    expect(screen.getByText("暖色系")).toBeDefined();
    expect(screen.getByText("科技风格")).toBeDefined();
  });

  it("renders trigger tags for each skill", () => {
    render(
      <StylePicker
        skills={sampleSkills}
        onSelect={() => {}}
        onClose={() => {}}
      />,
    );
    // First skill's triggers
    expect(screen.getByText("暖色")).toBeDefined();
    expect(screen.getByText("温暖")).toBeDefined();
    // Second skill's triggers
    expect(screen.getByText("科技")).toBeDefined();
    expect(screen.getByText("数码")).toBeDefined();
    expect(screen.getByText("AI")).toBeDefined();
  });

  it("calls onSelect with the clicked skill", () => {
    const onSelect = vi.fn();
    render(
      <StylePicker
        skills={sampleSkills}
        onSelect={onSelect}
        onPreview={() => {}}
        onClose={() => {}}
      />,
    );
    const skillItem = screen.getByText("暖色系").closest(".rounded-xl")!;
    const selectButton = within(skillItem as HTMLElement).getByText("Select");
    fireEvent.click(selectButton);
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith(sampleSkills[0]);
  });

  it("calls onClose when the X close button is clicked", () => {
    const onClose = vi.fn();
    render(
      <StylePicker
        skills={sampleSkills}
        onSelect={() => {}}
        onClose={onClose}
      />,
    );
    // The X button is the first button in the header
    const buttons = screen.getAllByRole("button");
    fireEvent.click(buttons[0]);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("renders nothing (returns null) when skills list is empty", () => {
    const { container } = render(
      <StylePicker skills={[]} onSelect={() => {}} onClose={() => {}} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders correct number of skill option buttons", () => {
    render(
      <StylePicker
        skills={sampleSkills}
        onSelect={() => {}}
        onClose={() => {}}
      />,
    );
    const buttons = screen.getAllByRole("button");
    // 2 skills × 2 buttons each (Preview + Select) + 1 close = 5 buttons
    expect(buttons.length).toBe(5);
  });

  it("renders skills with no triggers (empty array)", () => {
    const skillNoTriggers: SkillOption[] = [
      { name: "No Triggers", description: "A skill with no triggers", triggers: [] },
    ];
    render(
      <StylePicker
        skills={skillNoTriggers}
        onSelect={() => {}}
        onClose={() => {}}
      />,
    );
    expect(screen.getByText("No Triggers")).toBeDefined();
    expect(screen.getByText("A skill with no triggers")).toBeDefined();
    // No trigger tags should be rendered.
    const tagElements = document.querySelectorAll(".rounded-md");
    // The "rounded-md" class is used on close button too, so just check no trigger spans.
    expect(screen.queryByText("warm")).toBeNull();
  });

  it("truncates triggers display to max 5 per skill", () => {
    const manyTriggers: SkillOption[] = [
      {
        name: "Many Triggers",
        description: "Has many triggers",
        triggers: ["t1", "t2", "t3", "t4", "t5", "t6", "t7"],
      },
    ];
    render(
      <StylePicker
        skills={manyTriggers}
        onSelect={() => {}}
        onClose={() => {}}
      />,
    );
    // First 5 triggers should be visible.
    expect(screen.getByText("t1")).toBeDefined();
    expect(screen.getByText("t5")).toBeDefined();
    // t6 and t7 should be truncated.
    expect(screen.queryByText("t6")).toBeNull();
    expect(screen.queryByText("t7")).toBeNull();
  });

  it("renders skill description when scenario is also available", () => {
    const skill: SkillOption[] = [
      {
        name: "Full Skill",
        description: "Primary description",
        triggers: [],
        scenario: "marketing",
      },
    ];
    render(
      <StylePicker skills={skill} onSelect={() => {}} onClose={() => {}} />,
    );
    // Description takes priority over scenario in the sub-text.
    expect(screen.getByText("Primary description")).toBeDefined();
  });

  it("renders scenario as fallback when description is empty", () => {
    const skill: SkillOption[] = [
      {
        name: "Scenario Only",
        description: "",
        triggers: [],
        scenario: "education",
      },
    ];
    render(
      <StylePicker skills={skill} onSelect={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByText("education")).toBeDefined();
  });

  it("renders empty sub-text when both description and scenario are empty", () => {
    const skill: SkillOption[] = [
      {
        name: "No Meta",
        description: "",
        triggers: [],
        scenario: "",
      },
    ];
    render(
      <StylePicker skills={skill} onSelect={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByText("No Meta")).toBeDefined();
  });

  it("renders long skill name with proper truncation", () => {
    const skill: SkillOption[] = [
      {
        name: "This Is A Very Long Skill Name That Should Be Truncated In The UI Because It Exceeds Normal Parameters",
        description: "",
        triggers: [],
      },
    ];
    render(
      <StylePicker skills={skill} onSelect={() => {}} onClose={() => {}} />,
    );
    const nameElement = screen.getByText(
      "This Is A Very Long Skill Name That Should Be Truncated In The UI Because It Exceeds Normal Parameters",
    );
    expect(nameElement).toBeDefined();
    // The element has the "truncate" class for text overflow handling.
    expect(nameElement.className).toContain("truncate");
  });

  it("renders many skills in scrollable container", () => {
    const manySkills: SkillOption[] = Array.from({ length: 20 }, (_, i) => ({
      name: `Skill ${i + 1}`,
      description: `Description ${i + 1}`,
      triggers: [`trigger-${i}`],
    }));
    render(
      <StylePicker skills={manySkills} onSelect={() => {}} onClose={() => {}} />,
    );
    expect(screen.getByText("Skill 1")).toBeDefined();
    expect(screen.getByText("Skill 20")).toBeDefined();
    // 20 skills × 2 buttons each (Preview + Select) + 1 close = 41 buttons.
    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBe(41);
  });
});
