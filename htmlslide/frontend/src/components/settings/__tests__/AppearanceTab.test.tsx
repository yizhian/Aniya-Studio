import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AppearanceTab } from "../AppearanceTab";

const mockSetLocale = vi.fn();
const mockToggleTheme = vi.fn();

vi.mock("../../../context/LocaleContext", () => ({
  useLocale: () => ({
    t: {
      settings: { theme: "Theme", language: "Language" },
      theme: { light: "Light", dark: "Dark" },
    },
    locale: "en-US",
    setLocale: mockSetLocale,
  }),
}));

vi.mock("../../../hooks/useThemeMode", () => ({
  useThemeMode: () => ({
    themeMode: "dark",
    toggleThemeMode: mockToggleTheme,
  }),
}));

describe("AppearanceTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders theme toggle buttons", () => {
    render(<AppearanceTab />);
    expect(screen.getByText("Light")).toBeTruthy();
    expect(screen.getByText("Dark")).toBeTruthy();
  });

  it("renders language toggle buttons", () => {
    render(<AppearanceTab />);
    expect(screen.getByText("中文")).toBeTruthy();
    expect(screen.getByText("English")).toBeTruthy();
  });

  it("toggles theme when clicking the non-active theme button", () => {
    render(<AppearanceTab />);
    fireEvent.click(screen.getByText("Light"));
    expect(mockToggleTheme).toHaveBeenCalledTimes(1);
  });

  it("does not toggle when clicking the active theme button", () => {
    render(<AppearanceTab />);
    fireEvent.click(screen.getByText("Dark"));
    expect(mockToggleTheme).not.toHaveBeenCalled();
  });

  it("sets locale to zh-CN when clicking 中文", () => {
    render(<AppearanceTab />);
    fireEvent.click(screen.getByText("中文"));
    expect(mockSetLocale).toHaveBeenCalledWith("zh-CN");
  });

  it("sets locale to en-US when clicking English", () => {
    render(<AppearanceTab />);
    fireEvent.click(screen.getByText("English"));
    expect(mockSetLocale).toHaveBeenCalledWith("en-US");
  });
});
