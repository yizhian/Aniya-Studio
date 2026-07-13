import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import React, { useContext } from "react";
import {
  EditorRuntimeContext,
  EditorContext,
  EditorProvider,
  type EditorRuntimeValue,
} from "../EditorContext";

// A simple consumer to test context values
function ContextConsumer() {
  const runtime = useContext(EditorRuntimeContext);
  const editor = useContext(EditorContext);

  return (
    <div>
      <span data-testid="has-editor">
        {runtime.editor ? "yes" : "no"}
      </span>
      <span data-testid="legacy-editor">
        {editor ? "yes" : "no"}
      </span>
    </div>
  );
}

describe("EditorContext", () => {
  it("provides null editor by default (no provider)", () => {
    render(<ContextConsumer />);
    // Without provider, defaults are null
    expect(screen.getByTestId("has-editor").textContent).toBe("no");
    expect(screen.getByTestId("legacy-editor").textContent).toBe("no");
  });

  it("provides the editor instance from EditorProvider", () => {
    // EditorProvider requires projectId, mode, themeMode, locale props
    render(
      <EditorProvider projectId="test-project" mode="edit" themeMode="dark" locale="zh-CN">
        <ContextConsumer />
      </EditorProvider>
    );
    // The EditorProvider wraps children and provides the editor through context.
    // The mock grapesjs module (export default {}) allows a minimal editor instance.
    // Both the new and legacy contexts should provide a value.
    expect(screen.getByTestId("has-editor").textContent).toBe("yes");
    expect(screen.getByTestId("legacy-editor").textContent).toBe("yes");
  });

  it("EditorRuntimeContext default value has editor: null", () => {
    const TestComponent = () => null;
    render(<TestComponent />);

    let capturedValue: EditorRuntimeValue | undefined;
    render(
      <EditorRuntimeContext.Consumer>
        {(value) => {
          capturedValue = value;
          return null;
        }}
      </EditorRuntimeContext.Consumer>
    );

    expect(capturedValue).toEqual({ editor: null });
  });

  it("legacy EditorContext default value is null", () => {
    let capturedValue: unknown;
    render(
      <EditorContext.Consumer>
        {(value) => {
          capturedValue = value;
          return null;
        }}
      </EditorContext.Consumer>
    );

    expect(capturedValue).toBeNull();
  });

  it("both contexts are exported", () => {
    expect(EditorRuntimeContext).toBeDefined();
    expect(EditorContext).toBeDefined();
    expect(EditorProvider).toBeDefined();
  });
});
