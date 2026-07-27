import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { TabList, TabPanel, TabTrigger, Tabs } from "@/components/ui/tabs";

function TabsHarness() {
  return (
    <Tabs defaultValue="profile">
      <TabList aria-label="资料设置">
        <TabTrigger value="profile">资料</TabTrigger>
        <TabTrigger value="appearance">外观</TabTrigger>
      </TabList>
      <TabPanel value="profile">资料内容</TabPanel>
      <TabPanel value="appearance">外观内容</TabPanel>
    </Tabs>
  );
}

describe("Tabs", () => {
  it("exposes selected panels and supports arrow-key navigation", async () => {
    const user = userEvent.setup();
    render(<TabsHarness />);

    const profileTab = screen.getByRole("tab", { name: "资料" });
    expect(profileTab.getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("tabpanel").textContent).toContain("资料内容");

    profileTab.focus();
    await user.keyboard("{ArrowRight}");

    expect(screen.getByRole("tab", { name: "外观" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("tabpanel").textContent).toContain("外观内容");
  });
});
