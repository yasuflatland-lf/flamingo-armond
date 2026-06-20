import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Pencil, Trash2 } from "lucide-react";
import { Button } from "./button";
import { SplitButtonMenu } from "./split-button-menu";

const noop = () => undefined;

const meta = {
  title: "UI/SplitButtonMenu",
  component: SplitButtonMenu,
} satisfies Meta<typeof SplitButtonMenu>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    triggerLabel: "More actions",
    items: [],
  },
  render: () => (
    <div className="inline-flex">
      <Button variant="outline" size="sm" className="rounded-r-none">
        Edit
      </Button>
      <SplitButtonMenu
        triggerLabel="More actions"
        items={[
          { key: "rename", icon: <Pencil className="h-4 w-4" />, label: "Rename", onSelect: noop },
          {
            key: "delete",
            icon: <Trash2 className="h-4 w-4" />,
            label: "Delete",
            destructive: true,
            onSelect: noop,
          },
        ]}
      />
    </div>
  ),
};
