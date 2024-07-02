import React from "react";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import LoadingPage from "./LoadingPage";

describe("LoadingPage", () => {
  it("renders without crashing", () => {
    render(<LoadingPage />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("displays the flamingo icon with correct styling", () => {
    render(<LoadingPage />);
    const flamingoIcon = screen.getByTestId("flamingo-icon");
    expect(flamingoIcon).toBeInTheDocument();
    expect(flamingoIcon).toHaveClass("text-white text-8xl animate-flip");
  });

  it("displays the loading text with correct styling", () => {
    render(<LoadingPage />);
    const loadingText = screen.getByText("Loading...");
    expect(loadingText).toBeInTheDocument();
    expect(loadingText).toHaveClass("text-lg text-white mt-4");
  });

  it("has the correct background styling", () => {
    render(<LoadingPage />);
    const container = screen.getByTestId("loading-container");
    expect(container).toHaveClass(
      "bg-pink-700 flex flex-col items-center justify-center h-screen bg-gray-100",
    );
  });
});
