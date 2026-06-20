import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ErrorBanner } from "./error-banner";

const meta = {
  title: "UI/ErrorBanner",
  component: ErrorBanner,
  args: { children: "Something went wrong. Please try again." },
} satisfies Meta<typeof ErrorBanner>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const LongMessage: Story = {
  args: {
    children:
      "We could not save your changes because the connection was lost. Your edits are preserved locally — retry when you are back online.",
  },
};
