import { mount } from "@vue/test-utils";
import { nextTick } from "vue";
import { NCard, NInput, NModal } from "naive-ui";
import AppIconButton from "../components/AppIconButton.vue";
import AppSegmented from "../components/AppSegmented.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import OperationProgress from "../components/OperationProgress.vue";
import ReportViewer from "../components/ReportViewer.vue";
import SecretInput from "../components/SecretInput.vue";
import TextInput from "../components/TextInput.vue";
import DeepView from "./DeepView.vue";
import { useOperations } from "../composables/useOperations";
import { createTheme } from "../composables/useTheme";
import type { RouterAPI } from "../services/api";

function makeAPI(): RouterAPI {
  return {
    Doctor: vi.fn().mockResolvedValue({ status: "succeeded" }),
    Verify: vi.fn().mockResolvedValue({ status: "succeeded" }),
    EnableSSH: vi.fn().mockResolvedValue({ status: "succeeded" }),
    PersistSoft: vi.fn().mockResolvedValue({ status: "succeeded" }),
    PersistRemove: vi.fn().mockResolvedValue({ status: "succeeded" }),
    RebootVerify: vi.fn().mockResolvedValue({ status: "succeeded" }),
    BackupCreate: vi.fn().mockResolvedValue({ status: "succeeded" }),
    BackupVerify: vi.fn().mockResolvedValue({ status: "succeeded" }),
    BackupVerifyOnline: vi.fn().mockResolvedValue({ status: "succeeded" }),
    DeepPersist: vi.fn().mockResolvedValue({ status: "succeeded" }),
    Cancel: vi.fn().mockResolvedValue(true),
    ListReports: vi.fn().mockResolvedValue([]),
    ResolveHostKey: vi.fn(),
    OnProgress: vi.fn().mockReturnValue(() => {}),
    OnHostKey: vi.fn().mockReturnValue(() => {}),
  };
}

function mountDeep(store: any) {
  return mount(DeepView, {
    global: {
      provide: { operations: store },
      stubs: { RouterLink: { template: "<a><slot /></a>" } },
      components: { NModal, NCard, NInput },
    },
    attachTo: document.body,
  });
}

describe("OperationProgress", () => {
  it("collapses events so finished stages are not shown as running", () => {
    const wrapper = mount(OperationProgress, {
      props: {
        status: "failed",
        events: [
          { stage: "validate", status: "running" },
          { stage: "probe", status: "running" },
          { stage: "failed", status: "failed" },
        ],
      },
    });
    const rows = wrapper.findAll("li");
    expect(rows.map((row) => row.text())).toEqual(["validate成功", "probe失败"]);
  });
});

describe("ConfirmDialog", () => {
  it("labels the acknowledgement input, describes the dialog and puts actions last", async () => {
    const wrapper = mount(ConfirmDialog, {
      props: {
        modelValue: false,
        title: "最终深度固化确认",
        description: "写入会擦除分区。",
        requireText: "RD08-DEEP",
      },
      global: { components: { NModal, NCard, NInput } },
      attachTo: document.body,
    });
    await wrapper.setProps({ modelValue: true });
    await new Promise((resolve) => setTimeout(resolve, 60));
    const dialog = document.body.querySelector<HTMLElement>(".confirm-dialog")!;
    const label = dialog.querySelector<HTMLLabelElement>("label")!;
    const input = dialog.querySelector<HTMLInputElement>("input")!;
    expect(label.control).toBe(input);
    const describedBy = dialog.getAttribute("aria-describedby")!;
    expect(document.getElementById(describedBy)?.textContent).toContain("写入会擦除分区");
    const order = [...dialog.querySelectorAll("p, input, button")].map(
      (element) => element.tagName,
    );
    expect(order.at(-1)).toBe("BUTTON");
    expect(order.indexOf("INPUT")).toBeLessThan(order.indexOf("BUTTON"));
    wrapper.unmount();
  });

  it("uses monospace only when asked", async () => {
    const wrapper = mount(ConfirmDialog, {
      props: {
        modelValue: false,
        title: "未知 SSH 主机密钥",
        description: "主机：x",
        monoDescription: true,
      },
      global: { components: { NModal, NCard, NInput } },
      attachTo: document.body,
    });
    await wrapper.setProps({ modelValue: true });
    await new Promise((resolve) => setTimeout(resolve, 60));
    expect(document.body.querySelector(".confirm-dialog__description--mono")).not.toBeNull();
    wrapper.unmount();
  });
});

describe("form semantics", () => {
  it("marks required inputs for assistive technology", () => {
    const text = mount(TextInput, {
      props: { id: "host", label: "地址", modelValue: "", required: true },
    });
    expect(text.find("input").attributes("aria-required")).toBe("true");
    const secret = mount(SecretInput, {
      props: { id: "root", label: "Root 密码", modelValue: "", required: true },
    });
    expect(secret.find("input").attributes("aria-required")).toBe("true");
  });

  it("gives the selected segment a cue that is not colour alone", () => {
    const wrapper = mount(AppSegmented, {
      props: {
        modelValue: "dark",
        label: "主题",
        options: [
          { value: "light", label: "浅色" },
          { value: "dark", label: "深色" },
        ],
      },
    });
    const labels = wrapper.findAll("label");
    expect(labels[1].find("svg").exists()).toBe(true);
    expect(labels[0].find("svg").exists()).toBe(false);
  });

  it("shows a busy icon button", () => {
    const wrapper = mount(AppIconButton, {
      props: { label: "复制", loading: true },
      slots: { default: "<svg class='slot-icon' />" },
    });
    expect(wrapper.attributes("aria-busy")).toBe("true");
    expect(wrapper.attributes("disabled")).toBeDefined();
    expect(wrapper.find(".slot-icon").exists()).toBe(false);
  });
});

describe("ReportViewer", () => {
  it("keeps the visible label inside the accessible name and resets the copied state", async () => {
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
    const wrapper = mount(ReportViewer, { props: { report: { status: "failed" } } });
    const copy = wrapper.find('[aria-label="复制报告"]');
    await copy.trigger("click");
    await nextTick();
    const copied = wrapper.find("button");
    expect(copied.attributes("aria-label")).toContain(copied.text().trim());
    await wrapper.setProps({ report: { status: "succeeded" } });
    expect(wrapper.find("button").text()).toContain("复制报告");
    const download = wrapper.findAll("button")[1];
    expect(download.attributes("aria-label") ?? download.text()).toContain(download.text().trim());
  });
});

describe("theme switching", () => {
  it("suppresses transitions while the palette changes", async () => {
    const root = document.createElement("html");
    const theme = createTheme({ storage: null, matchMedia: null, root });
    theme.setPreference("dark");
    expect(root.classList.contains("rt-theme-switching")).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(root.classList.contains("rt-theme-switching")).toBe(false);
  });
});

describe("DeepView", () => {
  async function openStepTwo(wrapper: any) {
    await wrapper.find("#deep-backup").setValue("/backups/rd08");
    await wrapper.find("#deep-run").setValue("/private/run");
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("确认备份与恢复边界"))
      .trigger("click");
  }

  it("asks for its own Telnet acknowledgement even if Telnet was allowed elsewhere", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    store.allowTelnet = true;
    const wrapper = mountDeep(store);
    await openStepTwo(wrapper);
    const telnet = wrapper.find<HTMLInputElement>('input[role="switch"]');
    expect(telnet.element.checked).toBe(false);
    await wrapper.find("#deep-root").setValue("root");
    await wrapper.find("#deep-ack").setValue("RD08-DEEP");
    const final = () =>
      wrapper.findAll("button").find((button: any) => button.text().includes("进行最终确认"))!;
    expect(final().attributes("disabled")).toBeDefined();
    await telnet.setValue(true);
    expect(final().attributes("disabled")).toBeUndefined();
    wrapper.unmount();
  });

  it("keeps the non-retry instruction on the page and clears secrets when left", async () => {
    const store = useOperations(makeAPI());
    store.rootPassword = "root-secret";
    store.ackDeep = "RD08-DEEP";
    const wrapper = mountDeep(store);
    expect(wrapper.text()).toContain("不要从头重跑");
    expect(wrapper.text()).toContain("RECOVERY.md");
    wrapper.unmount();
    expect(store.rootPassword).toBe("");
    expect(store.ackDeep).toBe("");
  });
});

describe("per-form validation", () => {
  it("shows field errors and does not call Go for an invalid preflight", async () => {
    const { default: DashboardView } = await import("./DashboardView.vue");
    const api = makeAPI();
    const store = useOperations(api);
    store.target.host = "";
    store.withAdminAuth = true;
    const wrapper = mount(DashboardView, {
      global: {
        provide: { operations: store },
        stubs: { RouterLink: { template: "<a><slot /></a>" } },
      },
      attachTo: document.body,
    });
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("运行预检"))!
      .trigger("click");
    expect(api.Doctor).not.toHaveBeenCalled();
    const host = wrapper.find("#target-host");
    expect(host.attributes("aria-invalid")).toBe("true");
    expect(document.getElementById(host.attributes("aria-describedby")!)?.textContent).toContain(
      "地址",
    );
    expect(wrapper.find("#dashboard-admin").attributes("aria-invalid")).toBe("true");
    wrapper.unmount();
  });

  it("labels port tiles with the target the preflight ran against", async () => {
    const { default: DashboardView } = await import("./DashboardView.vue");
    const api = makeAPI();
    vi.mocked(api.Doctor).mockResolvedValue({
      status: "succeeded",
      host: "192.168.31.1",
      ssh: "open",
      telnet: "closed",
      admin_auth: "skipped",
      device: { rom_version: "1.1.96", hardware: "RD08" },
    });
    const store = useOperations(api);
    const wrapper = mount(DashboardView, {
      global: {
        provide: { operations: store },
        stubs: { RouterLink: { template: "<a><slot /></a>" } },
      },
      attachTo: document.body,
    });
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("运行预检"))!
      .trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 0));
    (store.target as any).web_port = "8080";
    await nextTick();
    expect(wrapper.text()).toContain("HTTP 80");
    expect(wrapper.text()).not.toContain("HTTP 8080");
    wrapper.unmount();
  });

  it("requires both passwords before the enable confirmation opens", async () => {
    const { default: EnableView } = await import("./EnableView.vue");
    const store = useOperations(makeAPI());
    const wrapper = mount(EnableView, {
      global: { provide: { operations: store }, components: { NModal, NCard, NInput } },
      attachTo: document.body,
    });
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("检查并开启 SSH"))!
      .trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 60));
    expect(document.body.querySelector(".confirm-dialog")).toBeNull();
    expect(wrapper.find("#enable-admin").attributes("aria-invalid")).toBe("true");
    expect(wrapper.find("#enable-root").attributes("aria-invalid")).toBe("true");
    wrapper.unmount();
  });
});
