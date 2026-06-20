import type { FormEvent } from "react";
import { describe, expect, it, vi } from "vitest";
import { submitFormHandler, wrapSubmit } from "./submit-handler";

function fakeEvent() {
  return {
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  } as unknown as FormEvent<HTMLFormElement>;
}

describe("submitFormHandler", () => {
  it("cancels the native submit and delegates to form.handleSubmit", () => {
    const handleSubmit = vi.fn(() => Promise.resolve());
    const e = fakeEvent();
    submitFormHandler({ handleSubmit })(e);
    expect(e.preventDefault).toHaveBeenCalledOnce();
    expect(e.stopPropagation).toHaveBeenCalledOnce();
    expect(handleSubmit).toHaveBeenCalledOnce();
  });

  it("swallows a rejected handleSubmit so it never becomes an unhandled rejection", async () => {
    const handleSubmit = vi.fn(() => Promise.reject(new Error("boom")));
    expect(() => submitFormHandler({ handleSubmit })(fakeEvent())).not.toThrow();
    // Let the swallowing microtask settle; an unhandled rejection here would fail the run.
    await Promise.resolve();
  });
});

describe("wrapSubmit", () => {
  it("passes the value through and resolves on success", async () => {
    const submit = vi.fn((_v: { name: string }) => Promise.resolve());
    await expect(wrapSubmit("x-form", submit)({ name: "ok" })).resolves.toBeUndefined();
    expect(submit).toHaveBeenCalledWith({ name: "ok" });
  });

  it("re-throws on rejection so formState.isSubmitSuccessful stays false", async () => {
    const submit = vi.fn(() => Promise.reject(new Error("nope")));
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    await expect(wrapSubmit("x-form", submit)(undefined as never)).rejects.toThrow("nope");
    errSpy.mockRestore();
  });

  it("redacts err.message from the console payload (logs only err.name)", async () => {
    const submit = vi.fn(() => Promise.reject(new TypeError("secret user input")));
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    await expect(wrapSubmit("x-form", submit)(undefined as never)).rejects.toBeInstanceOf(
      TypeError,
    );
    expect(errSpy).toHaveBeenCalledWith("[x-form] submit rejected", { name: "TypeError" });
    const payload = errSpy.mock.calls[0]?.[1] as Record<string, unknown> | undefined;
    expect(payload).not.toHaveProperty("message");
    errSpy.mockRestore();
  });

  it("labels a non-Error rejection as unknown", async () => {
    const submit = vi.fn(() => Promise.reject("plain string"));
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    await expect(wrapSubmit("x-form", submit)(undefined as never)).rejects.toBe("plain string");
    expect(errSpy).toHaveBeenCalledWith("[x-form] submit rejected", { name: "unknown" });
    errSpy.mockRestore();
  });
});
