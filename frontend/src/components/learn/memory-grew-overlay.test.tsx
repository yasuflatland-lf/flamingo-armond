// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryGrewOverlay } from "./memory-grew-overlay";

describe("MemoryGrewOverlay — positive delta", () => {
  it("renders Memory +88% for fromStability=2.5 toStability=21", () => {
    const onDone = vi.fn();
    render(<MemoryGrewOverlay fromStability={2.5} toStability={21} onDone={onDone} />);
    expect(screen.getByText("Memory +88%")).toBeInTheDocument();
  });
});

describe("MemoryGrewOverlay — no gain", () => {
  it("renders nothing visible when fromStability equals toStability", () => {
    const onDone = vi.fn();
    render(<MemoryGrewOverlay fromStability={21} toStability={21} onDone={onDone} />);
    expect(screen.queryByText(/Memory/)).not.toBeInTheDocument();
  });

  it("calls onDone when there is no gain", () => {
    const onDone = vi.fn();
    render(<MemoryGrewOverlay fromStability={21} toStability={21} onDone={onDone} />);
    expect(onDone).toHaveBeenCalled();
  });
});

describe("MemoryGrewOverlay — stability decrease", () => {
  it("renders nothing and calls onDone when stability decreases", () => {
    const onDone = vi.fn();
    render(<MemoryGrewOverlay fromStability={10} toStability={3} onDone={onDone} />);
    expect(screen.queryByText(/Memory/)).not.toBeInTheDocument();
    expect(onDone).toHaveBeenCalled();
  });
});

describe("MemoryGrewOverlay — auto-dismiss", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("calls onDone after 1500ms when delta is positive", () => {
    const onDone = vi.fn();
    render(<MemoryGrewOverlay fromStability={2.5} toStability={21} onDone={onDone} />);
    expect(onDone).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1500);
    expect(onDone).toHaveBeenCalledTimes(1);
  });
});
