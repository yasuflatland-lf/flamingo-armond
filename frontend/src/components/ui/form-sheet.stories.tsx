import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { Button } from "./button";
import { FormSheet } from "./form-sheet";
import { Input } from "./input";
import { Label } from "./label";

const meta = {
  title: "UI/FormSheet",
  component: FormSheet,
} satisfies Meta<typeof FormSheet>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    open: true,
    onOpenChange: () => undefined,
    title: "Edit card group",
    children: null,
  },
  render: () => {
    const [open, setOpen] = useState(true);
    return (
      <>
        <Button variant="outline" onClick={() => setOpen(true)}>
          Open form sheet
        </Button>
        <FormSheet
          open={open}
          onOpenChange={setOpen}
          title="Edit card group"
          description="Update the name and save your changes."
        >
          <div className="grid gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="fs-name">Name</Label>
              <Input id="fs-name" defaultValue="Biology 101" />
            </div>
            <Button variant="brand" onClick={() => setOpen(false)}>
              Save
            </Button>
          </div>
        </FormSheet>
      </>
    );
  },
};
