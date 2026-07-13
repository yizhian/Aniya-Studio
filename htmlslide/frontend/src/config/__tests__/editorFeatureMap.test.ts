import { describe, it, expect } from "vitest";
import { INSPECTOR_SECTIONS } from "../editorFeatureMap";

describe("INSPECTOR_SECTIONS", () => {
  it("has exactly 4 sections", () => {
    expect(INSPECTOR_SECTIONS).toHaveLength(4);
  });

  it("section ids are unique", () => {
    const ids = INSPECTOR_SECTIONS.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("section ids match expected names", () => {
    const ids = INSPECTOR_SECTIONS.map((s) => s.id);
    expect(ids).toEqual(["typography", "layout", "spacing", "effects"]);
  });

  it("every section has a label and icon", () => {
    for (const section of INSPECTOR_SECTIONS) {
      expect(section.label).toBeTruthy();
      expect(section.icon).toBeTruthy();
    }
  });

  describe("property ids are unique across all sections", () => {
    it("no duplicate property ids", () => {
      const allIds = INSPECTOR_SECTIONS.flatMap((s) =>
        s.properties.map((p) => p.id),
      );
      expect(new Set(allIds).size).toBe(allIds.length);
    });
  });

  describe("property styleKeys are unique across all sections", () => {
    it("no duplicate styleKeys", () => {
      const allKeys = INSPECTOR_SECTIONS.flatMap((s) =>
        s.properties.map((p) => p.styleKey),
      );
      expect(new Set(allKeys).size).toBe(allKeys.length);
    });
  });

  describe("typography section", () => {
    const typography = INSPECTOR_SECTIONS.find((s) => s.id === "typography")!;

    it("has 7 properties", () => {
      expect(typography.properties).toHaveLength(7);
    });

    it("has correct property ids", () => {
      expect(typography.properties.map((p) => p.id)).toEqual([
        "font-family",
        "font-size",
        "font-weight",
        "color",
        "text-align",
        "line-height",
        "letter-spacing",
      ]);
    });

    it("font-size has valid constraints", () => {
      const prop = typography.properties.find((p) => p.id === "font-size")!;
      expect(prop.constraints).toEqual({ min: 8, max: 200, step: 1, unit: "px" });
    });

    it("color property has type 'color'", () => {
      const prop = typography.properties.find((p) => p.id === "color")!;
      expect(prop.type).toBe("color");
    });

    it("font-family property has type 'fontFamily'", () => {
      const prop = typography.properties.find((p) => p.id === "font-family")!;
      expect(prop.type).toBe("fontFamily");
    });

    it("text-align has icon options", () => {
      const prop = typography.properties.find((p) => p.id === "text-align")!;
      expect(prop.options).toHaveLength(3);
      for (const opt of prop.options!) {
        expect(opt.icon).toBeTruthy();
      }
    });
  });

  describe("layout section", () => {
    const layout = INSPECTOR_SECTIONS.find((s) => s.id === "layout")!;

    it("has 7 properties", () => {
      expect(layout.properties).toHaveLength(7);
    });

    it("width and height have 0-9999 range", () => {
      const width = layout.properties.find((p) => p.id === "width")!;
      const height = layout.properties.find((p) => p.id === "height")!;
      expect(width.constraints!.max).toBe(9999);
      expect(height.constraints!.max).toBe(9999);
    });

    it("display has block/flex/grid/none options", () => {
      const prop = layout.properties.find((p) => p.id === "display")!;
      expect(prop.options!.map((o) => o.value)).toEqual([
        "block",
        "flex",
        "grid",
        "none",
      ]);
    });
  });

  describe("spacing section", () => {
    const spacing = INSPECTOR_SECTIONS.find((s) => s.id === "spacing")!;

    it("has 3 properties", () => {
      expect(spacing.properties).toHaveLength(3);
    });

    it("margin allows negative values", () => {
      const prop = spacing.properties.find((p) => p.id === "margin")!;
      expect(prop.constraints!.min).toBe(-500);
    });

    it("border-radius has non-negative range", () => {
      const prop = spacing.properties.find((p) => p.id === "border-radius")!;
      expect(prop.constraints!.min).toBe(0);
      expect(prop.constraints!.max).toBe(500);
    });
  });

  describe("effects section", () => {
    const effects = INSPECTOR_SECTIONS.find((s) => s.id === "effects")!;

    it("has 4 properties", () => {
      expect(effects.properties).toHaveLength(4);
    });

    it("opacity has 0-1 range with step 0.05", () => {
      const prop = effects.properties.find((p) => p.id === "opacity")!;
      expect(prop.constraints).toEqual({ min: 0, max: 1, step: 0.05 });
    });

    it("has two color properties (bg-color and border-color)", () => {
      const colorProps = effects.properties.filter((p) => p.type === "color");
      expect(colorProps).toHaveLength(2);
      expect(colorProps.map((p) => p.id)).toEqual([
        "background-color",
        "border-color",
      ]);
    });
  });

  describe("every property has required fields", () => {
    it("every property has id, label, type, styleKey", () => {
      for (const section of INSPECTOR_SECTIONS) {
        for (const prop of section.properties) {
          expect(prop.id).toBeTruthy();
          expect(prop.label).toBeTruthy();
          expect(prop.type).toBeTruthy();
          expect(prop.styleKey).toBeTruthy();
        }
      }
    });
  });

  describe("number-type properties have constraints", () => {
    it("all number properties define min and max", () => {
      for (const section of INSPECTOR_SECTIONS) {
        for (const prop of section.properties) {
          if (prop.type === "number") {
            expect(prop.constraints).toBeDefined();
            expect(typeof prop.constraints!.min).toBe("number");
            expect(typeof prop.constraints!.max).toBe("number");
          }
        }
      }
    });
  });

  describe("segmented properties have options", () => {
    it("all segmented properties have non-empty options", () => {
      for (const section of INSPECTOR_SECTIONS) {
        for (const prop of section.properties) {
          if (prop.type === "segmented") {
            expect(prop.options).toBeDefined();
            expect(prop.options!.length).toBeGreaterThan(0);
          }
        }
      }
    });
  });
});
