// Extend Vitest's expect with @testing-library/jest-dom matchers (toBeInTheDocument, etc.)
// This file is loaded via vitest.config.ts setupFiles for all test environments.
// jest-dom 6+ supports the vitest extend API when globals are available.
import "@testing-library/jest-dom/vitest";
import { Globals } from "@react-spring/web";

Globals.assign({ skipAnimation: true });
