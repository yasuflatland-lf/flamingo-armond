// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryGrewOverlay } from "./memory-grew-overlay";

describe("MemoryGrewOverlay — positive delta", () => {
	it("renders Memory +88% for from=2.5 to=21", () => {
		const onDone = vi.fn();
		render(<MemoryGrewOverlay from={2.5} to={21} onDone={onDone} />);
		expect(screen.getByText("Memory +88%")).toBeInTheDocument();
	});
});

describe("MemoryGrewOverlay — no gain", () => {
	it("renders nothing visible when from equals to", () => {
		const onDone = vi.fn();
		render(<MemoryGrewOverlay from={21} to={21} onDone={onDone} />);
		expect(screen.queryByText(/Memory/)).not.toBeInTheDocument();
	});

	it("calls onDone when there is no gain", () => {
		const onDone = vi.fn();
		render(<MemoryGrewOverlay from={21} to={21} onDone={onDone} />);
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
		render(<MemoryGrewOverlay from={2.5} to={21} onDone={onDone} />);
		expect(onDone).not.toHaveBeenCalled();
		vi.advanceTimersByTime(1500);
		expect(onDone).toHaveBeenCalledTimes(1);
	});
});
