import { mount } from "@vue/test-utils";
import AppButton from "./AppButton.vue";
import SecretInput from "./SecretInput.vue";
import StatusBadge from "./StatusBadge.vue";
import ReportViewer from "./ReportViewer.vue";
import OperationProgress from "./OperationProgress.vue";

describe("AppButton", () => {
  it("emits click when enabled", async () => {
    const wrapper = mount(AppButton, { slots: { default: "运行" } });
    await wrapper.trigger("click");
    expect(wrapper.emitted("click")).toHaveLength(1);
  });

  it("exposes loading and blocks activation", async () => {
    const wrapper = mount(AppButton, { props: { loading: true }, slots: { default: "运行中" } });
    expect(wrapper.attributes("aria-busy")).toBe("true");
    expect(wrapper.attributes("disabled")).toBeDefined();
    await wrapper.trigger("click");
    expect(wrapper.emitted("click")).toBeUndefined();
  });
});

describe("SecretInput", () => {
  it("links label and error and toggles secret visibility", async () => {
    const wrapper = mount(SecretInput, {
      props: { id: "root-password", label: "Root 密码", modelValue: "secret", error: "密码不正确" },
    });
    const input = wrapper.find("input");
    expect(input.attributes("type")).toBe("password");
    expect(input.attributes("autocomplete")).toBe("current-password");
    expect(input.attributes("aria-describedby")).toContain("root-password-error");
    expect(wrapper.find("label").text()).toBe("Root 密码");
    await wrapper.find("button").trigger("click");
    expect(input.attributes("type")).toBe("text");
    expect(wrapper.find("button").attributes("aria-label")).toBe("隐藏密码");
    await input.setValue("new-secret");
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual(["new-secret"]);
  });
});

describe("StatusBadge", () => {
  it.each([
    ["succeeded", "成功"],
    ["partial", "部分完成"],
    ["unknown", "结果未知"],
    ["failed", "失败"],
  ])("renders icon and text for %s", (status, text) => {
    const wrapper = mount(StatusBadge, { props: { status } });
    expect(wrapper.attributes("data-status")).toBe(status);
    expect(wrapper.text()).toContain(text);
    expect(wrapper.find("svg").exists()).toBe(true);
  });
});

describe("ReportViewer", () => {
  it("renders full formatted JSON and copy/download actions", () => {
    const wrapper = mount(ReportViewer, {
      props: { report: { status: "partial", message: "x".repeat(500) } },
    });
    expect(wrapper.find("pre").text()).toContain('"status": "partial"');
    expect(wrapper.find("pre").text().length).toBeGreaterThan(500);
    expect(wrapper.find('[aria-label="复制报告"]').exists()).toBe(true);
    expect(wrapper.find('[aria-label="下载报告"]').exists()).toBe(true);
  });
});

describe("OperationProgress", () => {
  it("shows stage evidence and a non-retry warning for unknown", () => {
    const wrapper = mount(OperationProgress, {
      props: {
        status: "unknown",
        events: [
          { stage: "PRECHECK", status: "succeeded" },
          { stage: "CRASH_WRITE_STARTED", status: "unknown" },
        ],
      },
    });
    expect(wrapper.text()).toContain("CRASH_WRITE_STARTED");
    expect(wrapper.text()).toContain("不要从头重跑");
  });
});
