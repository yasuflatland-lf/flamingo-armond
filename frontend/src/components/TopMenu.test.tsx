import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import React from "react";
import TopMenu from "./TopMenu";
import { BrowserRouter } from "react-router-dom";
import { vi } from "vitest";

// Mock fetch
global.fetch = vi.fn(() =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ results: [] }),
      headers: new Headers(),
      redirected: false,
      status: 200,
      statusText: "OK",
      type: "basic",
      url: "",
      clone: () => ({}),
      body: null,
      bodyUsed: false,
      arrayBuffer: () => Promise.resolve(new ArrayBuffer(0)),
      blob: () => Promise.resolve(new Blob()),
      formData: () => Promise.resolve(new FormData()),
      text: () => Promise.resolve(""),
    } as Response),
);

describe("TopMenu Component", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("renders search input and settings button", () => {
    render(
      <BrowserRouter>
        <TopMenu />
      </BrowserRouter>,
    );

    const inputElement = screen.getByPlaceholderText(/search.../i);
    const settingsButton = screen.getByRole("button");
    const searchIcon = screen.getByTestId("search-icon");
    const settingsIcon = screen.getByTestId("settings-icon");

    expect(inputElement).toBeInTheDocument();
    expect(settingsButton).toBeInTheDocument();
    expect(searchIcon).toBeInTheDocument();
    expect(settingsIcon).toBeInTheDocument();
  });

  test("updates search term on input change", () => {
    render(
      <BrowserRouter>
        <TopMenu />
      </BrowserRouter>,
    );

    const inputElement = screen.getByPlaceholderText(/search.../i);

    fireEvent.change(inputElement, { target: { value: "test" } });

    expect(inputElement).toHaveValue("test");
  });

  test("calls API on Enter key press", async () => {
    render(
      <BrowserRouter>
        <TopMenu />
      </BrowserRouter>,
    );

    const inputElement = screen.getByPlaceholderText(/search.../i);

    fireEvent.change(inputElement, { target: { value: "test" } });
    fireEvent.keyDown(inputElement, { key: "Enter", code: "Enter" });

    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1));
    expect(global.fetch).toHaveBeenCalledWith(
      "https://api.example.com/search",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ query: "test" }),
      },
    );
  });

  test("handles API call failure gracefully", async () => {
    (global.fetch as jest.Mock).mockImplementationOnce(() =>
        Promise.resolve({
          ok: false,
          headers: new Headers(),
          redirected: false,
          status: 500,
          statusText: "Internal Server Error",
          type: "basic",
          url: "",
          clone: () => ({}),
          body: null,
          bodyUsed: false,
          arrayBuffer: () => Promise.resolve(new ArrayBuffer(0)),
          blob: () => Promise.resolve(new Blob()),
          formData: () => Promise.resolve(new FormData()),
          text: () => Promise.resolve(""),
          json: () => Promise.resolve({}),
        } as Response),
    );

    render(
      <BrowserRouter>
        <TopMenu />
      </BrowserRouter>,
    );

    const inputElement = screen.getByPlaceholderText(/search.../i);

    fireEvent.change(inputElement, { target: { value: "test" } });
    fireEvent.keyDown(inputElement, { key: "Enter", code: "Enter" });

    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1));

    await waitFor(() =>
      expect(consoleErrorSpy).toHaveBeenCalledWith("API call failed"),
    );

    consoleErrorSpy.mockRestore();
  });

  test("handles network error gracefully", async () => {
    (global.fetch as jest.Mock).mockImplementationOnce(() =>
      Promise.reject(new Error("Network error")),
    );

    render(
      <BrowserRouter>
        <TopMenu />
      </BrowserRouter>,
    );

    const inputElement = screen.getByPlaceholderText(/search.../i);

    fireEvent.change(inputElement, { target: { value: "test" } });
    fireEvent.keyDown(inputElement, { key: "Enter", code: "Enter" });

    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    await waitFor(() =>
      expect(consoleErrorSpy).toHaveBeenCalledWith("Error:", expect.any(Error)),
    );

    consoleErrorSpy.mockRestore();
  });
});
