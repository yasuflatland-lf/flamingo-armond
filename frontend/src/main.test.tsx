import React from "react";
import {render} from "@testing-library/react";
import '@testing-library/jest-dom';
import Router from "./Router";
import {describe, expect, it, vi} from 'vitest';

// Mocking ReactDOM.createRoot since it doesn't work in a test environment
vi.mock('react-dom/client', () => ({
    createRoot: vi.fn().mockReturnValue({
        render: vi.fn()
    })
}));

describe('main.tsx', () => {
    it('renders Router component without crashing', () => {
        const {container} = render(
            <React.StrictMode>
                <Router/>
            </React.StrictMode>
        );
        expect(container).toBeInTheDocument();
    });
});
