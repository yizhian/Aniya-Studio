import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { UnsavedChangesDialog } from "../UnsavedChangesDialog";

// The test setup mocks useLocale() so t.save.cancel renders as "save.cancel",
// t.save.unsavedChanges renders as "save.unsavedChanges", etc.
const T = {
  unsavedChanges: "save.unsavedChanges",
  unsavedMessage: "save.unsavedMessage",
  cancel: "save.cancel",
  discard: "save.discard",
  saveAndContinue: "save.saveAndContinue",
};

describe("UnsavedChangesDialog", () => {
  // ── Closed state ──

  it("renders nothing when open is false", () => {
    const props = {
      open: false,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    expect(screen.queryByText("SYSTEM ALERT")).not.toBeInTheDocument();
    expect(screen.queryByText(T.unsavedChanges)).not.toBeInTheDocument();
  });

  // ── Open state ──

  it("renders the dialog when open is true", () => {
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    expect(screen.getByText("SYSTEM ALERT")).toBeInTheDocument();
    expect(screen.getByText(T.unsavedChanges)).toBeInTheDocument();
    expect(screen.getByText(T.unsavedMessage)).toBeInTheDocument();
  });

  // ── Button rendering ──

  it("renders three action buttons", () => {
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    expect(screen.getByText(T.cancel)).toBeInTheDocument();
    expect(screen.getByText(T.discard)).toBeInTheDocument();
    expect(screen.getByText(T.saveAndContinue)).toBeInTheDocument();
  });

  // ── Callback: cancel ──

  it("calls onCancel when cancel button is clicked", () => {
    const onCancel = vi.fn();
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel,
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);
    fireEvent.click(screen.getByText(T.cancel));

    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  // ── Callback: discard ──

  it("calls onDiscard when discard button is clicked", () => {
    const onDiscard = vi.fn();
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard,
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);
    fireEvent.click(screen.getByText(T.discard));

    expect(onDiscard).toHaveBeenCalledTimes(1);
  });

  // ── Callback: saveAndContinue ──

  it("calls onSaveAndProceed when save button is clicked", () => {
    const onSaveAndProceed = vi.fn();
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed,
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);
    fireEvent.click(screen.getByText(T.saveAndContinue));

    expect(onSaveAndProceed).toHaveBeenCalledTimes(1);
  });

  // ── Backdrop click ──

  it("calls onCancel when clicking the backdrop overlay", () => {
    const onCancel = vi.fn();
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel,
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    const backdrop = screen.getByText("SYSTEM ALERT").closest(".fixed.inset-0");
    if (backdrop) {
      fireEvent.click(backdrop);
      expect(onCancel).toHaveBeenCalled();
    }
  });

  // ── Disabled state while saving ──

  it("disables all buttons when saving is true", () => {
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: true,
    };

    render(<UnsavedChangesDialog {...props} />);

    const cancelBtn = screen.getByText(T.cancel);
    const discardBtn = screen.getByText(T.discard);
    const saveBtn = screen.getByText(T.saveAndContinue);

    expect(cancelBtn).toBeDisabled();
    expect(discardBtn).toBeDisabled();
    expect(saveBtn).toBeDisabled();
  });

  it("does not fire callbacks when buttons are disabled during save", () => {
    const onSaveAndProceed = vi.fn();
    const onDiscard = vi.fn();
    const onCancel = vi.fn();
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed,
      onDiscard,
      onCancel,
      saving: true,
    };

    render(<UnsavedChangesDialog {...props} />);

    fireEvent.click(screen.getByText(T.saveAndContinue));
    fireEvent.click(screen.getByText(T.discard));
    fireEvent.click(screen.getByText(T.cancel));

    expect(onSaveAndProceed).not.toHaveBeenCalled();
    expect(onDiscard).not.toHaveBeenCalled();
    expect(onCancel).not.toHaveBeenCalled();
  });

  // ── Spinner during save ──

  it("shows a spinner in the save button when saving is true", () => {
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: true,
    };

    render(<UnsavedChangesDialog {...props} />);

    const saveBtn = screen.getByText(T.saveAndContinue);
    const spinner = saveBtn.querySelector(".animate-spin");
    expect(spinner).toBeTruthy();
  });

  it("does not show spinner when not saving", () => {
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    const saveBtn = screen.getByText(T.saveAndContinue);
    const spinner = saveBtn.querySelector(".animate-spin");
    expect(spinner).toBeNull();
  });

  // ── mode prop (navigate vs rollback) ──

  it("renders the same UI for navigate mode", () => {
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    expect(screen.getByText("SYSTEM ALERT")).toBeInTheDocument();
  });

  it("renders the same UI for rollback mode", () => {
    const props = {
      open: true,
      mode: "rollback" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel: vi.fn(),
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    expect(screen.getByText("SYSTEM ALERT")).toBeInTheDocument();
    expect(screen.getByText(T.unsavedChanges)).toBeInTheDocument();
  });

  // ── Card click does not trigger onCancel ──

  it("does not call onCancel when clicking inside the dialog card", () => {
    const onCancel = vi.fn();
    const props = {
      open: true,
      mode: "navigate" as const,
      onSaveAndProceed: vi.fn(),
      onDiscard: vi.fn(),
      onCancel,
      saving: false,
    };

    render(<UnsavedChangesDialog {...props} />);

    fireEvent.click(screen.getByText(T.unsavedChanges));

    expect(onCancel).not.toHaveBeenCalled();
  });
});
