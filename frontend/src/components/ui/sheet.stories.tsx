import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Button } from "./button";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "./sheet";

const meta = {
  title: "UI/Sheet",
  component: Sheet,
} satisfies Meta<typeof Sheet>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {
  render: () => (
    <Sheet defaultOpen>
      <SheetTrigger asChild>
        <Button variant="outline">Open sheet</Button>
      </SheetTrigger>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Edit card group</SheetTitle>
          <SheetDescription>Update the name and save your changes.</SheetDescription>
        </SheetHeader>
        <SheetClose asChild>
          <Button variant="brand">Save</Button>
        </SheetClose>
      </SheetContent>
    </Sheet>
  ),
};
