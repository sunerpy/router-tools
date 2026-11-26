import { mount } from "@vue/test-utils";
import { nextTick, ref } from "vue";
import { NCard, NInput, NModal } from "naive-ui";
import AppIconButton from "../components/AppIconButton.vue";
import AppSegmented from "../components/AppSegmented.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import OperationProgress from "../components/OperationProgress.vue";
import ReportViewer from "../components/ReportViewer.vue";
import SecretInput from "../components/SecretInput.vue";
import TextInput from "../components/TextInput.vue";
import DeepView from "./DeepView.vue";
import SettingsView from "./SettingsView.vue";
import ToolsView from "./ToolsView.vue";
import { formatTime } from "../services/summary";
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
    GuidePlan: vi.fn(),
    GuideApply: vi.fn(),
    AcknowledgeRecovery: vi.fn(),
    DeepStatus: vi.fn(),
    CredentialStatus: vi.fn(),
    ForgetCredential: vi.fn(),
    ForgetAllCredentials: vi.fn(),
    Catalog: vi.fn().mockResolvedValue([]),
    Cancel: vi.fn().mockResolvedValue(true),
    ListReports: vi.fn().mockResolvedValue([]),
    SelectDirectory: vi.fn().mockResolvedValue(""),
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

function deepStatus(overrides: Record<string, unknown> = {}) {
  return {
    schema_version: 1,
    operation: "deep status",
    status: "succeeded",
    host: "192.168.31.1",
    device: { device_id: "D", rom_version: "1.1.96", new_encrypt_mode: 1 },
    profile: {
      id: "acme-r1-1.0",
      model_id: "acme-r1",
      vendor: "Acme",
      model: "R1",
      hardware: "R1",
      firmware: "1.0",
      capabilities: [],
      deep: { acknowledgement: "ACME-DEEP", reboot_transitions: 2 },
    },
    journal_path: "/managed/deep/acme-r1/192.168.31.1-abc/192.168.31.1-deep.json",
    journal_exists: false,
    backups: [{ path: "/managed/backups/b1", created_at: "2026-11-26T01:00:00Z", boot_id: "b" }],
    root_ssh: "not_checked",
    blocked: [],
    ready: false,
    ...overrides,
  };
}

function mountWith(
  component: any,
  store: any,
  api: RouterAPI,
  extra: Record<string, unknown> = {},
) {
  return mount(component, {
    global: {
      provide: { operations: store, api, catalog: ref([]), ...extra },
      stubs: { RouterLink: { template: "<a><slot /></a>" } },
      components: { NModal, NCard, NInput },
    },
    attachTo: document.body,
  });
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
  await nextTick();
}

describe("DeepView", () => {
  async function readStatus(wrapper: any) {
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("读取状态"))!
      .trigger("click");
    await flush();
  }
  async function openStepTwo(wrapper: any) {
    await readStatus(wrapper);
    await wrapper.find('input[name="deep-backup-choice"]').setValue(true);
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("确认备份与恢复边界"))!
      .trigger("click");
  }

  it("takes the phrase and reboot count from the resolved profile and never prefills them", async () => {
    const api = makeAPI();
    vi.mocked(api.DeepStatus).mockResolvedValue(deepStatus() as any);
    const store = useOperations(api);
    const wrapper = mountWith(DeepView, store, api);
    await openStepTwo(wrapper);
    expect(wrapper.text()).toContain("ACME-DEEP");
    expect(wrapper.text()).toContain("2 次重启");
    expect(wrapper.text()).not.toContain("RD08-DEEP");
    expect(wrapper.find<HTMLInputElement>("#deep-ack").element.value).toBe("");
    expect(wrapper.text()).toContain(
      "/managed/deep/acme-r1/192.168.31.1-abc/192.168.31.1-deep.json",
    );
    expect(store.backupDir).toBe("/managed/backups/b1");
    wrapper.unmount();
  });

  it("never preselects a backup", async () => {
    const api = makeAPI();
    vi.mocked(api.DeepStatus).mockResolvedValue(deepStatus() as any);
    const store = useOperations(api);
    const wrapper = mountWith(DeepView, store, api);
    await readStatus(wrapper);
    expect(store.backupDir).toBe("");
    expect(wrapper.find<HTMLInputElement>('input[name="deep-backup-choice"]').element.checked).toBe(
      false,
    );
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("确认备份与恢复边界"))!
      .trigger("click");
    expect(wrapper.find("#deep-root").exists()).toBe(false);
    wrapper.unmount();
  });

  it("tells a closed SSH port and a refused root password apart from a missing one", async () => {
    const api = makeAPI();
    vi.mocked(api.DeepStatus).mockResolvedValueOnce(deepStatus({ ssh: "closed" }) as any);
    const store = useOperations(api);
    const wrapper = mountWith(DeepView, store, api);
    await readStatus(wrapper);
    expect(wrapper.text()).toContain("SSH 端口没有开放");
    vi.mocked(api.DeepStatus).mockResolvedValueOnce(
      deepStatus({
        ssh: "open",
        root_ssh: "auth_failed",
        credentials: [{ kind: "root", action: "rejected" }],
      }) as any,
    );
    await readStatus(wrapper);
    expect(wrapper.text()).toContain("root 密码被拒绝");
    expect(wrapper.text()).toContain("已保存的密码被路由器拒绝");
    vi.mocked(api.DeepStatus).mockResolvedValueOnce(deepStatus({ ssh: "open" }) as any);
    await readStatus(wrapper);
    expect(wrapper.text()).toContain("未检查（需要 root 密码）");
    wrapper.unmount();
  });

  it("clears the deep secrets and the Telnet acknowledgement when the final confirmation is canceled", async () => {
    const api = makeAPI();
    vi.mocked(api.DeepStatus).mockResolvedValue(deepStatus() as any);
    const store = useOperations(api);
    const wrapper = mountWith(DeepView, store, api);
    await openStepTwo(wrapper);
    await wrapper.find("#deep-root").setValue("root-secret");
    await wrapper.find("#deep-ack").setValue("ACME-DEEP");
    const switches = wrapper.findAll<HTMLInputElement>('input[role="switch"]');
    await switches[switches.length - 1].setValue(true);
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("进行最终确认"))!
      .trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 60));
    (document.body.querySelector('[data-testid="confirm-cancel"]') as HTMLButtonElement).click();
    await flush();
    expect(store.rootPassword).toBe("");
    expect(store.ackDeep).toBe("");
    expect(wrapper.find<HTMLInputElement>("#deep-root").element.value).toBe("");
    const after = wrapper.findAll<HTMLInputElement>('input[role="switch"]');
    expect(after[after.length - 1].element.checked).toBe(false);
    expect(api.DeepPersist).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("asks for its own Telnet acknowledgement and always sends the managed journal directory", async () => {
    const api = makeAPI();
    vi.mocked(api.DeepStatus).mockResolvedValue(deepStatus() as any);
    const store = useOperations(api);
    store.allowTelnet = true;
    store.runDir = "/custom/run";
    const wrapper = mountWith(DeepView, store, api);
    await openStepTwo(wrapper);
    const telnet = wrapper.find<HTMLInputElement>('input[role="switch"]');
    expect(telnet.element.checked).toBe(true);
    const switches = wrapper.findAll<HTMLInputElement>('input[role="switch"]');
    const telnetAck = switches[switches.length - 1];
    expect(telnetAck.element.checked).toBe(false);
    await wrapper.find("#deep-root").setValue("root");
    await wrapper.find("#deep-ack").setValue("ACME-DEEP");
    const final = () =>
      wrapper.findAll("button").find((button: any) => button.text().includes("进行最终确认"))!;
    expect(final().attributes("disabled")).toBeDefined();
    await telnetAck.setValue(true);
    expect(final().attributes("disabled")).toBeUndefined();
    await final().trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 60));
    const ack = document.body.querySelector<HTMLInputElement>(".confirm-dialog input");
    ack!.value = "ACME-DEEP";
    ack!.dispatchEvent(new Event("input"));
    await nextTick();
    (document.body.querySelector('[data-testid="confirm-danger"]') as HTMLButtonElement).click();
    await flush();
    const request = vi.mocked(api.DeepPersist).mock.calls[0][0];
    expect(request.RunDir).toBe("");
    expect(request.AllowTelnet).toBe(true);
    expect(request.UseSavedPassword).toBe(false);
    expect(request.AckDeep).toBe("ACME-DEEP");
    wrapper.unmount();
  });

  it("keeps the non-retry instruction on the page and clears secrets when left", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    store.rootPassword = "root-secret";
    store.ackDeep = "ACME-DEEP";
    const wrapper = mountWith(DeepView, store, api);
    expect(wrapper.text()).toContain("不要从头重跑");
    expect(wrapper.text()).toContain("RECOVERY.md");
    wrapper.unmount();
    expect(store.rootPassword).toBe("");
    expect(store.ackDeep).toBe("");
  });
});

describe("GuideView", () => {
  async function mountGuide(planOverrides: Record<string, unknown> = {}) {
    const { default: GuideView } = await import("./GuideView.vue");
    const { useGuide } = await import("../composables/useGuide");
    const api = makeAPI();
    vi.mocked(api.CredentialStatus).mockResolvedValue({
      schema_version: 1,
      operation: "credentials list",
      status: "succeeded",
      entries: [],
    });
    vi.mocked(api.GuidePlan).mockResolvedValue({
      schema_version: 1,
      operation: "guide plan",
      status: "succeeded",
      host: "192.168.31.1",
      device: { device_id: "D", hardware: "RD08", rom_version: "1.1.96", new_encrypt_mode: 1 },
      profile: {
        id: "xiaomi-rd08-1.1.96",
        model_id: "xiaomi-rd08",
        vendor: "Xiaomi",
        model: "BE6500 Pro",
        hardware: "RD08",
        firmware: "1.1.96",
        capabilities: [],
      },
      goals: ["ssh", "backup", "soft"],
      state: {
        ssh: "closed",
        root_ssh: "not_checked",
        recovery_blocked: false,
        locked: false,
        saved_admin: false,
        saved_root: false,
      },
      steps: [{ id: "enable_ssh", action: "run", reason: "ssh_closed", effects: ["nvram_commit"] }],
      ready: true,
      changes: true,
      required_inputs: ["admin_password", "new_root_password"],
      confirmation: {
        schema_version: 1,
        target: { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 },
        identity: {
          device_id: "D",
          hardware: "RD08",
          rom_version: "1.1.96",
          profile_id: "xiaomi-rd08-1.1.96",
        },
        goals: ["ssh", "backup", "soft"],
        reset_root_password: false,
        steps: [{ id: "enable_ssh", action: "run", effects: ["nvram_commit"] }],
      },
      plan_digest: "ab".repeat(32),
      ...planOverrides,
    } as any);
    const store = useOperations(api);
    const guide = useGuide(api, store);
    const wrapper = mountWith(GuideView, store, api, { guide });
    return { wrapper, api, store, guide };
  }
  const click = async (wrapper: any, label: string) => {
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes(label))!
      .trigger("click");
    await flush();
  };

  it("opens with saved passwords allowed and remembering off, whatever the last visit chose", async () => {
    const { default: GuideView } = await import("./GuideView.vue");
    const { useGuide } = await import("../composables/useGuide");
    const api = makeAPI();
    vi.mocked(api.CredentialStatus).mockResolvedValue({
      schema_version: 1,
      operation: "credentials list",
      status: "succeeded",
      entries: [],
    });
    const store = useOperations(api);
    const guide = useGuide(api, store);
    guide.remember = true;
    guide.useSaved = false;
    mountWith(GuideView, store, api, { guide });
    await flush();
    expect(guide.remember).toBe(false);
    expect(guide.useSaved).toBe(true);
  });

  it("validates the address before detecting", async () => {
    const { wrapper, api, store } = await mountGuide();
    store.target.host = "";
    await click(wrapper, "检测");
    expect(api.GuidePlan).not.toHaveBeenCalled();
    const host = wrapper.find("#target-host");
    expect(host.attributes("aria-invalid")).toBe("true");
    expect(document.getElementById(host.attributes("aria-describedby")!)?.textContent).toContain(
      "地址",
    );
    wrapper.unmount();
  });

  it("requires the passwords the plan needs before the one confirmation opens", async () => {
    const { wrapper, api } = await mountGuide();
    await click(wrapper, "检测");
    await click(wrapper, "执行");
    await new Promise((resolve) => setTimeout(resolve, 60));
    expect(document.body.querySelector(".confirm-dialog")).toBeNull();
    expect(wrapper.find("#guide-admin").attributes("aria-invalid")).toBe("true");
    expect(wrapper.find("#guide-new-root").attributes("aria-invalid")).toBe("true");
    expect(api.GuideApply).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("keeps the detected target when the form is edited afterwards", async () => {
    const { wrapper, store } = await mountGuide();
    await click(wrapper, "检测");
    store.target.host = "10.0.0.9";
    await nextTick();
    expect(wrapper.text()).toContain("192.168.31.1");
    wrapper.unmount();
  });

  it("offers the acknowledgement when detection is blocked by an uncertain change", async () => {
    const { wrapper, api } = await mountGuide({
      status: "failed",
      ready: false,
      confirmation: undefined,
      plan_digest: undefined,
      error: { code: "RECOVERY_REVIEW_REQUIRED", message: "blocked" },
      state: {
        root_ssh: "not_checked",
        recovery_blocked: true,
        locked: false,
        saved_admin: false,
        saved_root: false,
        recovery: {
          schema_version: 1,
          operation: "enable",
          status: "unknown",
          device_write_possible: true,
          started_at: "",
        },
      },
      steps: [{ id: "enable_ssh", action: "blocked", reason: "recovery_review_required" }],
    });
    vi.mocked(api.AcknowledgeRecovery).mockResolvedValue({
      schema_version: 1,
      operation: "recovery ack",
      status: "succeeded",
      host: "192.168.31.1",
      acknowledged: true,
    });
    await click(wrapper, "检测");
    expect(wrapper.text()).toContain("上次变更的结果不确定");
    await click(wrapper, "我已检查路由器");
    await new Promise((resolve) => setTimeout(resolve, 60));
    (document.body.querySelector('[data-testid="confirm-danger"]') as HTMLButtonElement).click();
    await flush();
    expect(api.AcknowledgeRecovery).toHaveBeenCalledTimes(1);
    wrapper.unmount();
  });

  it("clears the passwords when the one confirmation is canceled", async () => {
    const { wrapper, guide, api } = await mountGuide();
    await click(wrapper, "检测");
    await wrapper.find("#guide-admin").setValue("admin-secret");
    await wrapper.find("#guide-new-root").setValue("new-root");
    await wrapper.find("#guide-new-root-repeat").setValue("new-root");
    await click(wrapper, "执行");
    await new Promise((resolve) => setTimeout(resolve, 60));
    (document.body.querySelector('[data-testid="confirm-cancel"]') as HTMLButtonElement).click();
    await flush();
    expect([guide.adminPassword, guide.newRootPassword, guide.newRootRepeat]).toEqual(["", "", ""]);
    expect(wrapper.find<HTMLInputElement>("#guide-admin").element.value).toBe("");
    expect(api.GuideApply).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("names the router in the confirmation and stops offering the run once the form changes", async () => {
    const { wrapper, store } = await mountGuide();
    await click(wrapper, "检测");
    await wrapper.find("#guide-admin").setValue("admin-secret");
    await wrapper.find("#guide-new-root").setValue("new-root");
    await wrapper.find("#guide-new-root-repeat").setValue("new-root");
    await click(wrapper, "执行");
    await new Promise((resolve) => setTimeout(resolve, 60));
    const dialog = document.body.querySelector(".confirm-dialog")!.textContent ?? "";
    expect(dialog).toContain("192.168.31.1");
    expect(dialog).toContain("xiaomi-rd08-1.1.96");
    (document.body.querySelector('[data-testid="confirm-cancel"]') as HTMLButtonElement).click();
    await flush();
    store.target.host = "10.0.0.9";
    await nextTick();
    expect(wrapper.findAll("button").some((button: any) => button.text() === "执行")).toBe(false);
    expect(wrapper.text()).toContain("检测之后改了设置");
    wrapper.unmount();
  });

  it("shows why a run failed before its first step, and a rejected run", async () => {
    const { wrapper, api } = await mountGuide();
    vi.mocked(api.GuideApply).mockResolvedValueOnce({
      schema_version: 1,
      operation: "guide apply",
      status: "failed",
      host: "192.168.31.1",
      steps: [],
      error: { code: "INVALID_PASSWORD", message: "the Web admin password is required" },
    } as any);
    await click(wrapper, "检测");
    await wrapper.find("#guide-admin").setValue("admin-secret");
    await wrapper.find("#guide-new-root").setValue("new-root");
    await wrapper.find("#guide-new-root-repeat").setValue("new-root");
    await click(wrapper, "执行");
    await new Promise((resolve) => setTimeout(resolve, 60));
    (document.body.querySelector('[data-testid="confirm-danger"]') as HTMLButtonElement).click();
    await flush();
    expect(wrapper.text()).toContain("INVALID_PASSWORD");
    wrapper.unmount();

    const rejected = await mountGuide();
    vi.mocked(rejected.api.GuideApply).mockRejectedValueOnce(new Error("duplicate operation"));
    await click(rejected.wrapper, "检测");
    await rejected.wrapper.find("#guide-admin").setValue("admin-secret");
    await rejected.wrapper.find("#guide-new-root").setValue("new-root");
    await rejected.wrapper.find("#guide-new-root-repeat").setValue("new-root");
    await click(rejected.wrapper, "执行");
    await new Promise((resolve) => setTimeout(resolve, 60));
    (document.body.querySelector('[data-testid="confirm-danger"]') as HTMLButtonElement).click();
    await flush();
    expect(rejected.wrapper.text()).toContain("duplicate operation");
    rejected.wrapper.unmount();
  });

  it("lets a running guided flow be canceled", async () => {
    const { wrapper, api } = await mountGuide();
    vi.mocked(api.GuideApply).mockReturnValue(new Promise(() => {}) as any);
    await click(wrapper, "检测");
    await wrapper.find("#guide-admin").setValue("admin-secret");
    await wrapper.find("#guide-new-root").setValue("new-root");
    await wrapper.find("#guide-new-root-repeat").setValue("new-root");
    await click(wrapper, "执行");
    await new Promise((resolve) => setTimeout(resolve, 60));
    (document.body.querySelector('[data-testid="confirm-danger"]') as HTMLButtonElement).click();
    await flush();
    const id = vi.mocked(api.GuideApply).mock.calls[0][0].OperationID;
    await click(wrapper, "取消运行");
    expect(api.Cancel).toHaveBeenCalledWith(id);
    wrapper.unmount();
  });

  it("offers a profile switch only for an exact registered match, on click", async () => {
    const candidate = (exact: boolean) => ({
      profile: {
        id: "xiaomi-rd08-1.1.96",
        model_id: "xiaomi-rd08",
        vendor: "Xiaomi",
        model: "BE6500 Pro",
        hardware: "RD08",
        firmware: "1.1.96",
        capabilities: [],
      },
      exact,
    });
    const unsupported = (exact: boolean) => ({
      status: "failed",
      ready: false,
      confirmation: undefined,
      plan_digest: undefined,
      error: { code: "UNSUPPORTED_FIRMWARE", message: "not registered" },
      candidates: [candidate(exact)],
      steps: [],
    });
    const { wrapper, api, store } = await mountGuide(unsupported(true));
    await click(wrapper, "检测");
    expect(store.profileID).toBe("auto");
    expect(api.GuidePlan).toHaveBeenCalledTimes(1);
    await click(wrapper, "切换到 xiaomi-rd08-1.1.96");
    expect(store.profileID).toBe("xiaomi-rd08-1.1.96");
    expect(api.GuidePlan).toHaveBeenCalledTimes(2);
    wrapper.unmount();

    const inexact = await mountGuide(unsupported(false));
    await click(inexact.wrapper, "检测");
    expect(inexact.wrapper.text()).toContain("1.1.96");
    expect(
      inexact.wrapper.findAll("button").some((button: any) => button.text().includes("切换到")),
    ).toBe(false);
    inexact.wrapper.unmount();
  });

  it("names the router and the recorded times in the recovery panel and shows a failed acknowledgement", async () => {
    const started = "2026-11-20T08:00:00Z";
    const finished = "2026-11-20T08:05:00Z";
    const { wrapper, api } = await mountGuide({
      status: "failed",
      ready: false,
      confirmation: undefined,
      plan_digest: undefined,
      error: { code: "RECOVERY_REVIEW_REQUIRED", message: "blocked" },
      state: {
        root_ssh: "not_checked",
        recovery_blocked: true,
        locked: false,
        saved_admin: false,
        saved_root: false,
        recovery: {
          schema_version: 1,
          operation: "enable",
          status: "unknown",
          device_write_possible: true,
          started_at: started,
          finished_at: finished,
        },
      },
      steps: [],
    });
    vi.mocked(api.AcknowledgeRecovery).mockResolvedValue({
      schema_version: 1,
      operation: "recovery ack",
      status: "failed",
      host: "192.168.31.1",
      acknowledged: false,
      error: { code: "STATE_STORE_FAILED", message: "disk full" },
    } as any);
    await click(wrapper, "检测");
    expect(wrapper.text()).toContain(formatTime(started));
    expect(wrapper.text()).toContain(formatTime(finished));
    await click(wrapper, "我已检查路由器");
    await new Promise((resolve) => setTimeout(resolve, 60));
    expect(document.body.querySelector(".confirm-dialog")!.textContent).toContain("192.168.31.1");
    (document.body.querySelector('[data-testid="confirm-danger"]') as HTMLButtonElement).click();
    await flush();
    expect(wrapper.text()).toContain("disk full");
    wrapper.unmount();
  });

  it("marks goals that were not selected instead of reporting them missing", async () => {
    const { wrapper } = await mountGuide({
      goals: ["ssh"],
      state: {
        ssh: "closed",
        root_ssh: "not_checked",
        recovery_blocked: false,
        locked: false,
        saved_admin: false,
        saved_root: false,
      },
    });
    await click(wrapper, "检测");
    const facts = wrapper.find(".guide__facts").text();
    expect(facts).toContain("软固化未选择");
    expect(facts).toContain("托管备份未选择");
    wrapper.unmount();
  });
});

describe("ToolsView", () => {
  it("clears the root password when the removal confirmation is canceled", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    const wrapper = mountWith(ToolsView, store, api);
    await wrapper.find("#tools-root").setValue("root-secret");
    await wrapper
      .findAll("button")
      .find((button: any) => button.text().includes("移除软固化"))!
      .trigger("click");
    await new Promise((resolve) => setTimeout(resolve, 60));
    (document.body.querySelector('[data-testid="confirm-cancel"]') as HTMLButtonElement).click();
    await flush();
    expect(store.rootPassword).toBe("");
    expect(api.PersistRemove).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});

describe("SettingsView", () => {
  it("keeps a failed forget visible after the list reloads", async () => {
    const api = makeAPI();
    const entry = {
      kind: "root",
      scope: "xiaomi-rd08",
      host: "192.168.31.1",
      port: 22,
      saved_at: "2026-11-26T01:00:00Z",
    };
    vi.mocked(api.CredentialStatus).mockResolvedValue({
      schema_version: 1,
      operation: "credentials list",
      status: "succeeded",
      entries: [entry],
    } as any);
    vi.mocked(api.ForgetCredential).mockResolvedValue({
      schema_version: 1,
      operation: "credentials forget",
      status: "failed",
      entries: [],
      error: { code: "CREDENTIAL_STORE_FAILED", message: "vault locked" },
    } as any);
    const store = useOperations(api);
    const root = document.createElement("div");
    const theme = createTheme({ storage: null, matchMedia: null, root });
    const wrapper = mountWith(SettingsView, store, api, { theme });
    await flush();
    await wrapper.find('button[aria-label^="忘记"]').trigger("click");
    await flush();
    expect(api.ForgetCredential).toHaveBeenCalledTimes(1);
    expect(wrapper.text()).toContain("vault locked");
    wrapper.unmount();
  });
});
