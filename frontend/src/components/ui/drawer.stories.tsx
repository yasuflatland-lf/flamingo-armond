import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { Button } from "./button";
import { Drawer, DrawerContent, DrawerDescription, DrawerHeader, DrawerTitle } from "./drawer";

const meta = {
  title: "UI/Drawer",
  component: Drawer,
} satisfies Meta<typeof Drawer>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => {
    const [open, setOpen] = useState(false);
    return (
      <>
        <Button variant="outline" onClick={() => setOpen(true)}>
          Open drawer
        </Button>
        <Drawer open={open} onOpenChange={setOpen}>
          <DrawerContent>
            <DrawerHeader>
              <DrawerTitle>Card group options</DrawerTitle>
              <DrawerDescription>Choose an action for this group.</DrawerDescription>
            </DrawerHeader>
            <div className="p-4">
              <Button variant="brand" onClick={() => setOpen(false)}>
                Confirm
              </Button>
            </div>
          </DrawerContent>
        </Drawer>
      </>
    );
  },
};
