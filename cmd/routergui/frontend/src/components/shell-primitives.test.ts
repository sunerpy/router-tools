import { mount } from "@vue/test-utils";
import AppBadge from "./AppBadge.vue";
import AppIconButton from "./AppIconButton.vue";
import AppSegmented from "./AppSegmented.vue";
import AppStepper from "./AppStepper.vue";
import AppToggle from "./AppToggle.vue";
import HistoryList from "./HistoryList.vue";
import StatTile from "./StatTile.vue";
import StatusBadge from "./StatusBadge.vue";

describe("StatusBadge labels", () => {
  it.each([
    ["running", "运行中", "info"],
    ["skipped", "已跳过", "neutral"],
    ["idle", "未运行", "neutral"],
    ["canceling", "正在取消", "neutral"],
    ["canceled", "已取消", "neutral"],
    ["failed", "失败", "danger"],
    ["partial", "部分完成", "warn"],
  ])("labels %s in Chinese with the %s tone", (status, text, tone) => {
    const wrapper = mount(StatusBadge, { props: { status } });
    expect(wrapper.text()).toBe(text);
    expect(wrapper.attributes("data-status")).toBe(status);
    expect(wrapper.attributes("data-tone")).toBe(tone);
    expect(wrapper.find("svg").exists()).toBe(true);
  });

  it("shows an unknown raw status as neutral text", () => {
    const wrapper = mount(StatusBadge, { props: { status: "weird" } });
    expect(wrapper.text()).toBe("weird");
    expect(wrapper.attributes("data-tone")).toBe("neutral");
  });
});

describe("AppBadge", () => {
  it("renders icon and text with a tone", () => {
    const wrapper = mount(AppBadge, { props: { tone: "ok" }, slots: { default: "精确匹配" } });
    expect(wrapper.text()).toBe("精确匹配");
    expect(wrapper.attributes("data-tone")).toBe("ok");
    expect(wrapper.find("svg").exists()).toBe(true);
  });
});

describe("AppToggle", () => {
  it("is a labelled native switch that emits its new state", async () => {
    const wrapper = mount(AppToggle, {
      props: { modelValue: false, note: "明文" },
      slots: { default: "允许 Telnet" },
    });
    const input = wrapper.find("input");
    expect(input.attributes("type")).toBe("checkbox");
    expect(input.attributes("role")).toBe("switch");
    expect((input.element as HTMLInputElement).checked).toBe(false);
    expect(wrapper.find("label").text()).toContain("允许 Telnet");
    expect(wrapper.text()).toContain("明文");
    await input.setValue(true);
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual([true]);
  });

  it("reflects a checked value and disables input", () => {
    const wrapper = mount(AppToggle, {
      props: { modelValue: true, disabled: true },
      slots: { default: "x" },
    });
    const input = wrapper.find("input").element as HTMLInputElement;
    expect(input.checked).toBe(true);
    expect(input.disabled).toBe(true);
  });
});

describe("AppSegmented", () => {
  const options = [
    { value: "system", label: "跟随系统" },
    { value: "light", label: "浅色" },
    { value: "dark", label: "深色" },
  ];

  it("is a named radio group that emits the chosen value", async () => {
    const wrapper = mount(AppSegmented, { props: { modelValue: "light", options, label: "外观" } });
    expect(wrapper.attributes("role")).toBe("radiogroup");
    expect(wrapper.attributes("aria-label")).toBe("外观");
    const radios = wrapper.findAll('input[type="radio"]');
    expect(radios).toHaveLength(3);
    expect((radios[1].element as HTMLInputElement).checked).toBe(true);
    await radios[2].setValue(true);
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual(["dark"]);
  });
});

describe("AppIconButton", () => {
  it("requires an accessible name and emits click", async () => {
    const wrapper = mount(AppIconButton, {
      props: { label: "复制路径" },
      slots: { default: "<svg />" },
    });
    expect(wrapper.attributes("aria-label")).toBe("复制路径");
    expect(wrapper.attributes("title")).toBe("复制路径");
    await wrapper.trigger("click");
    expect(wrapper.emitted("click")).toHaveLength(1);
  });
});

describe("StatTile", () => {
  it("renders title, meta, value and sentence with its tone", () => {
    const wrapper = mount(StatTile, {
      props: {
        tone: "info",
        title: "SSH",
        meta: "22",
        value: "端口开放",
        detail: "开放端口不算认证",
      },
    });
    expect(wrapper.attributes("data-tone")).toBe("info");
    for (const text of ["SSH", "22", "端口开放", "开放端口不算认证"]) {
      expect(wrapper.text()).toContain(text);
    }
  });
});

describe("AppStepper", () => {
  it("marks completed, current and pending steps", () => {
    const wrapper = mount(AppStepper, {
      props: { steps: ["前置证据", "凭据与明文风险", "最终确认"], current: 2 },
    });
    const items = wrapper.findAll("li");
    expect(items).toHaveLength(3);
    expect(items[0].attributes("data-state")).toBe("done");
    expect(items[1].attributes("aria-current")).toBe("step");
    expect(items[1].attributes("data-state")).toBe("current");
    expect(items[2].attributes("data-state")).toBe("pending");
    expect(wrapper.text()).toContain("最终确认");
  });
});

describe("HistoryList", () => {
  const rows = [
    {
      id: "e1",
      operation: "enable",
      title: "开启 SSH",
      meta: "10:15:30 · 192.168.31.1",
      status: "partial",
    },
    {
      id: "d1",
      operation: "doctor",
      title: "设备预检",
      meta: "09:00:00 · 192.168.31.1",
      status: "succeeded",
    },
  ];

  it("renders rows with status badges", () => {
    const wrapper = mount(HistoryList, { props: { rows } });
    expect(wrapper.findAll("li")).toHaveLength(2);
    expect(wrapper.text()).toContain("开启 SSH");
    expect(wrapper.text()).toContain("部分完成");
    expect(wrapper.text()).toContain("10:15:30 · 192.168.31.1");
  });

  it("emits the selected row and marks it when selectable", async () => {
    const wrapper = mount(HistoryList, { props: { rows, selectable: true, selected: "d1" } });
    const buttons = wrapper.findAll("button.history-list__row");
    expect(buttons).toHaveLength(2);
    expect(buttons[1].attributes("aria-current")).toBe("true");
    await buttons[0].trigger("click");
    expect(wrapper.emitted("select")?.at(-1)).toEqual(["e1"]);
  });
});
