import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Input } from "./input";
import { Label } from "./label";

const meta = {
  title: "UI/Input",
  component: Input,
  args: { placeholder: "Type here" },
} satisfies Meta<typeof Input>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Disabled: Story = { args: { disabled: true } };
export const WithValue: Story = { args: { defaultValue: "Existing value" } };
export const WithLabel: Story = {
  render: (args) => (
    <div className="grid w-72 gap-1.5">
      <Label htmlFor="story-input">Email</Label>
      <Input id="story-input" type="email" {...args} />
    </div>
  ),
};
