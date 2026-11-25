import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import { NCard, NInput, NModal } from "naive-ui";
import ConfirmDialog from "./ConfirmDialog.vue";
import AppAlert from "./AppAlert.vue";
import AppCard from "./AppCard.vue";
import EmptyState from "./EmptyState.vue";

describe("ConfirmDialog", () => {
  it("requires exact acknowledgement and keeps destructive confirm disabled", async () => {
    const wrapper = mount(ConfirmDialog, {
      props: { modelValue: false, title: "深度固化", requireText: "RD08-DEEP" },
      global: { components: { NModal, NCard, NInput } },
      attachTo: document.body,
    });
    await wrapper.setProps({ modelValue: true });
    await new Promise((resolve) => setTimeout(resolve, 60));
    const confirm = document.body.querySelector<HTMLButtonElement>(
      '[data-testid="confirm-danger"]',
    )!;
    const cancel = document.body.querySelector<HTMLButtonElement>(
      '[data-testid="confirm-cancel"]',
    )!;
    const input = document.body.querySelector<HTMLInputElement>(".confirm-dialog input")!;
    expect(confirm.disabled).toBe(true);
    expect(cancel).not.toBeNull();
    input.value = "wrong";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await nextTick();
    expect(confirm.disabled).toBe(true);
    input.value = "RD08-DEEP";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await nextTick();
    expect(confirm.disabled).toBe(false);
    confirm.click();
    expect(wrapper.emitted("confirm")).toHaveLength(1);
  });
});

describe("surface primitives", () => {
  it("renders alert semantics and actionable content", () => {
    const wrapper = mount(AppAlert, {
      props: { type: "error", title: "失败", message: "请检查报告后重试" },
    });
    expect(wrapper.attributes("role")).toBe("alert");
    expect(wrapper.text()).toContain("请检查报告后重试");
  });

  it("renders card regions", () => {
    const wrapper = mount(AppCard, {
      props: { title: "设备信息" },
      slots: { default: "<p>RD08</p>", actions: "<button>运行</button>" },
    });
    expect(wrapper.text()).toContain("设备信息");
    expect(wrapper.text()).toContain("RD08");
    expect(wrapper.text()).toContain("运行");
  });

  it("renders an actionable empty state", () => {
    const wrapper = mount(EmptyState, {
      props: { title: "尚无报告", message: "先运行设备预检。" },
    });
    expect(wrapper.text()).toContain("先运行设备预检。");
  });
});
