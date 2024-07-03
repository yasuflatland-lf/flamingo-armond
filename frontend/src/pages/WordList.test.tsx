import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MemoryRouter } from "react-router-dom";
import WordList from "./WordList";

// Mock data for people
const mockPeople = [
  {
    name: "Leslie Alexander",
    email: "leslie.alexander@example.com",
    role: "Co-Founder / CEO",
    imageUrl:
      "https://images.unsplash.com/photo-1494790108377-be9c29b29330?ixlib=rb-1.2.1&ixid=eyJhcHBfaWQiOjEyMDd9&auto=format&fit=facearea&facepad=2&w=256&h=256&q=80",
    lastSeen: "3h ago",
    lastSeenDateTime: "2023-01-23T13:23Z",
  },
  {
    name: "Michael Foster",
    email: "michael.foster@example.com",
    role: "Co-Founder / CTO",
    imageUrl:
      "https://images.unsplash.com/photo-1519244703995-f4e0f30006d5?ixlib=rb-1.2.1&ixid=eyJhcHBfaWQiOjEyMDd9&auto=format&fit=facearea&facepad=2&w=256&h=256&q=80",
    lastSeen: "3h ago",
    lastSeenDateTime: "2023-01-23T13:23Z",
  },
  // Add more mock people if necessary
];

// Helper function to render the component
const renderComponent = () => {
  return render(
    <MemoryRouter>
      <WordList />
    </MemoryRouter>,
  );
};

describe("WordList Component", () => {
  beforeEach(() => {
    // Mock the fetch function
    global.fetch = vi.fn(() =>
      Promise.resolve({
        json: () => Promise.resolve(mockPeople),
      }),
    ) as jest.Mock;
  });

  it("renders the component and shows the loading state initially", () => {
    renderComponent();
    expect(screen.getByText(/Showing/i)).toBeInTheDocument();
  });

  it("fetches and displays data correctly", async () => {
    renderComponent();

    // Wait for the data to be fetched and rendered
    await waitFor(() => {
      mockPeople.forEach((person) => {
        expect(screen.getByText(person.name)).toBeInTheDocument();
        expect(screen.getByText(person.email)).toBeInTheDocument();
        expect(screen.getByText(person.role)).toBeInTheDocument();
      });
    });
  });

  it("displays online status correctly when lastSeen is null", async () => {
    const mockPeopleWithOnlineStatus = [
      ...mockPeople,
      {
        name: "Tom Cook",
        email: "tom.cook@example.com",
        role: "Director of Product",
        imageUrl:
          "https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?ixlib=rb-1.2.1&ixid=eyJhcHBfaWQiOjEyMDd9&auto=format&fit=facearea&facepad=2&w=256&h=256&q=80",
        lastSeen: null,
      },
    ];

    global.fetch = vi.fn(() =>
      Promise.resolve({
        json: () => Promise.resolve(mockPeopleWithOnlineStatus),
      }),
    ) as jest.Mock;

    renderComponent();

    // Wait for the data to be fetched and rendered
    await waitFor(() => {
      expect(screen.getByText("Tom Cook")).toBeInTheDocument();
      expect(screen.getByText("Online")).toBeInTheDocument();
    });
  });
});
