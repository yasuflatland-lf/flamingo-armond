import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
} from "./sidebar";

const meta = {
  title: "UI/Sidebar",
  component: Sidebar,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Sidebar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader className="px-3 py-2 text-sm font-semibold">flamingo</SidebarHeader>
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton isActive>Card groups</SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton>Catalog</SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton>Learn</SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter className="px-3 py-2 text-xs text-muted-foreground">Signed in</SidebarFooter>
      </Sidebar>
      <SidebarInset>
        <div className="p-6">Main content area</div>
      </SidebarInset>
    </SidebarProvider>
  ),
};
