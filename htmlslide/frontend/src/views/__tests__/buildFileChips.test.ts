import { describe, it, expect } from "vitest";
import { buildFileChips } from "../../viewmodels/useHomeViewModel";

function fakeFile(name: string): File {
  return new File([""], name);
}

describe("buildFileChips", () => {
  it("maps file names to chip files", () => {
    const chips = buildFileChips(
      [fakeFile("doc.pdf"), fakeFile("img.png")],
      [],
      "uploading",
    );
    expect(chips).toHaveLength(2);
    expect(chips[0].name).toBe("doc.pdf");
    expect(chips[1].name).toBe("img.png");
    expect(chips[0].status).toBe("uploading");
    expect(chips[1].status).toBe("uploading");
  });

  it("sets status to 'done' when upload status is done", () => {
    const chips = buildFileChips([fakeFile("doc.pdf")], [], "done");
    expect(chips[0].status).toBe("done");
  });

  it("sets status to 'error' when upload status is error", () => {
    const chips = buildFileChips([fakeFile("doc.pdf")], [], "error");
    expect(chips[0].status).toBe("error");
  });

  it("sets status to 'parsing' when upload status is parsing", () => {
    const chips = buildFileChips([fakeFile("doc.pdf")], [], "parsing");
    expect(chips[0].status).toBe("parsing");
  });

  it("maps parsed file metadata to chip fields", () => {
    const parsed = [
      {
        original_name: "doc.pdf",
        type: "pdf",
        saved_path_rel: "/tmp/doc.pdf",
        pages: 10,
        char_count: 5000,
      },
    ];
    const chips = buildFileChips([fakeFile("doc.pdf")], parsed, "done");
    expect(chips[0].type).toBe("pdf");
    expect(chips[0].pages).toBe(10);
    expect(chips[0].charCount).toBe(5000);
  });

  it("handles parsed error field", () => {
    const parsed = [
      {
        original_name: "bad.pdf",
        type: "error",
        saved_path_rel: "",
        error: "Unsupported format",
      },
    ];
    const chips = buildFileChips([fakeFile("bad.pdf")], parsed, "error");
    expect(chips[0].error).toBe("Unsupported format");
  });

  it("handles empty files array", () => {
    const chips = buildFileChips([], [], "done");
    expect(chips).toHaveLength(0);
  });

  it("handles missing parsed entry gracefully (index beyond parsed)", () => {
    const chips = buildFileChips(
      [fakeFile("a.pdf"), fakeFile("b.pdf")],
      [], // no parsed entries
      "uploading",
    );
    expect(chips).toHaveLength(2);
    expect(chips[0].type).toBeUndefined();
    expect(chips[0].pages).toBeUndefined();
    expect(chips[0].charCount).toBeUndefined();
  });

  it("handles idle status", () => {
    const chips = buildFileChips([fakeFile("doc.pdf")], [], "idle");
    expect(chips[0].status).toBe("idle");
  });
});
