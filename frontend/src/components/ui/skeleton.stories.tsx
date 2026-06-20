import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Skeleton } from "./skeleton";

const meta = {
  title: "UI/Skeleton",
  component: Skeleton,
} satisfies Meta<typeof Skeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Line: Story = { args: { className: "h-4 w-48" } };
export const Avatar: Story = { args: { className: "h-12 w-12 rounded-full" } };
export const Card: Story = {
  render: () => (
    <div className="flex flex-col gap-2">
      <Skeleton className="h-24 w-64" />
      <Skeleton className="h-4 w-48" />
      <Skeleton className="h-4 w-32" />
    </div>
  ),
};
