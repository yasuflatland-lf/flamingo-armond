import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { toast } from "sonner";
import { Button } from "./button";
import { Toaster } from "./sonner";

const meta = {
  title: "UI/Toaster",
  component: Toaster,
} satisfies Meta<typeof Toaster>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <>
      <Toaster />
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={() => toast("Card group saved")}>
          Default toast
        </Button>
        <Button variant="outline" onClick={() => toast.success("Imported successfully")}>
          Success
        </Button>
        <Button variant="outline" onClick={() => toast.error("Could not save")}>
          Error
        </Button>
      </div>
    </>
  ),
};
