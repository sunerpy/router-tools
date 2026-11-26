import { mount } from "@vue/test-utils";
import HistoryList from "./HistoryList.vue";

describe("HistoryList", () => {
  it("marks rows an agent started with an Agent badge before the status", () => {
    const wrapper = mount(HistoryList, {
      props: {
        rows: [
          {
            id: "agent-1",
            operation: "backup_create",
            title: "创建备份",
            meta: "09:59:00",
            status: "running",
            agent: true,
          },
          {
            id: "desktop-1",
            operation: "doctor",
            title: "设备预检",
            meta: "09:58:00",
            status: "succeeded",
          },
        ],
      },
    });
    const rows = wrapper.findAll("li");
    expect(rows[0].text()).toContain("Agent");
    expect(rows[0].find('[data-tone="info"]').exists()).toBe(true);
    expect(rows[1].text()).not.toContain("Agent");
  });
});
