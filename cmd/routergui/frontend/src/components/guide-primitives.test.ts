import { mount } from "@vue/test-utils";
import CredentialState from "./CredentialState.vue";
import DeviceSelector from "./DeviceSelector.vue";
import PlanStepList from "./PlanStepList.vue";
import StatusBadge from "./StatusBadge.vue";
import type { CatalogEntry, GuideStep, GuideStepResult, Profile } from "../services/api";

const profile = (firmware: string): Profile => ({
  id: `xiaomi-rd08-${firmware}`,
  model_id: "xiaomi-rd08",
  vendor: "Xiaomi",
  model: "BE6500 Pro",
  hardware: "RD08",
  firmware,
  capabilities: [],
});

const catalog: CatalogEntry[] = [
  {
    model_id: "xiaomi-rd08",
    vendor: "Xiaomi",
    model: "BE6500 Pro",
    hardware: "RD08",
    profiles: [profile("1.1.96"), profile("1.1.97")],
  },
];

describe("DeviceSelector", () => {
  it("offers automatic detection or one exact firmware of a model", async () => {
    const wrapper = mount(DeviceSelector, { props: { catalog, modelValue: "auto", id: "device" } });
    const model = wrapper.find<HTMLSelectElement>("#device-model");
    expect(model.findAll("option").map((option) => option.text())).toEqual([
      "自动识别",
      "Xiaomi BE6500 Pro（RD08）",
    ]);
    expect(wrapper.find<HTMLSelectElement>("#device-firmware").element.disabled).toBe(true);

    await model.setValue("xiaomi-rd08");
    expect(wrapper.emitted("update:modelValue")!.at(-1)).toEqual(["xiaomi-rd08-1.1.96"]);
    await wrapper.setProps({ modelValue: "xiaomi-rd08-1.1.96" });
    await wrapper.find("#device-firmware").setValue("xiaomi-rd08-1.1.97");
    expect(wrapper.emitted("update:modelValue")!.at(-1)).toEqual(["xiaomi-rd08-1.1.97"]);
    await wrapper.setProps({ modelValue: "xiaomi-rd08-1.1.97" });
    await model.setValue("auto");
    expect(wrapper.emitted("update:modelValue")!.at(-1)).toEqual(["auto"]);
  });

  it("shows the stored exact profile as selected", () => {
    const wrapper = mount(DeviceSelector, {
      props: { catalog, modelValue: "xiaomi-rd08-1.1.97", id: "device" },
    });
    expect(wrapper.find<HTMLSelectElement>("#device-model").element.value).toBe("xiaomi-rd08");
    expect(wrapper.find<HTMLSelectElement>("#device-firmware").element.value).toBe(
      "xiaomi-rd08-1.1.97",
    );
  });

  it("links a model error to the model select", () => {
    const wrapper = mount(DeviceSelector, {
      props: { catalog, modelValue: "", id: "device", error: "请选择设备型号" },
    });
    const model = wrapper.find("#device-model");
    expect(model.attributes("aria-invalid")).toBe("true");
    expect(model.attributes("aria-describedby")).toContain("device-model-error");
    expect(wrapper.find("#device-model-error").text()).toBe("请选择设备型号");
  });

  it("flags a stored profile that is no longer registered and keeps it", () => {
    const wrapper = mount(DeviceSelector, {
      props: { catalog, modelValue: "xiaomi-rd08-9.9.9", id: "device" },
    });
    expect(wrapper.text()).toContain("xiaomi-rd08-9.9.9");
    expect(wrapper.text()).toContain("未注册");
    expect(wrapper.emitted("update:modelValue")).toBeUndefined();
  });
});

describe("PlanStepList", () => {
  const steps: GuideStep[] = [
    {
      id: "enable_ssh",
      action: "run",
      reason: "ssh_closed",
      effects: ["nvram_commit", "root_password_change"],
    },
    { id: "backup", action: "skip", reason: "backup_verified" },
    {
      id: "soft_install",
      action: "blocked",
      reason: "soft_uci_drift",
      maintenance: "persist_remove",
    },
  ];

  it("lists each planned step with its action, reason and effects in words", () => {
    const wrapper = mount(PlanStepList, { props: { steps } });
    const rows = wrapper.findAll("ol > li");
    expect(rows).toHaveLength(3);
    expect(rows[0].text()).toContain("开启 SSH");
    expect(rows[0].text()).toContain("将执行");
    expect(rows[0].text()).toContain("NVRAM");
    expect(rows[0].text()).toContain("root 密码改为");
    expect(rows[1].text()).toContain("已满足");
    expect(rows[2].text()).toContain("已阻止");
    expect(rows[2].text()).toContain("移除软固化");
  });

  it("shows each step's result status and the failing step's error", () => {
    const results: GuideStepResult[] = [
      { id: "enable_ssh", action: "run", status: "succeeded" },
      {
        id: "backup",
        action: "run",
        status: "failed",
        error: { code: "BACKUP_INCOMPLETE", message: "short read" },
      },
      { id: "soft_install", action: "not_run", status: "not_run" },
    ];
    const wrapper = mount(PlanStepList, { props: { results } });
    const rows = wrapper.findAll("ol > li");
    expect(rows[0].text()).toContain("成功");
    expect(rows[1].text()).toContain("BACKUP_INCOMPLETE");
    expect(rows[2].text()).toContain("未执行");
  });
});

describe("CredentialState", () => {
  it("names the kind and state in words and never a value", () => {
    expect(mount(CredentialState, { props: { kind: "web-admin", state: "saved" } }).text()).toBe(
      "Web 管理密码已保存",
    );
    expect(mount(CredentialState, { props: { kind: "root", state: "rejected" } }).text()).toBe(
      "root 密码已被拒绝",
    );
    expect(mount(CredentialState, { props: { kind: "root", state: "none" } }).text()).toBe(
      "root 密码未保存",
    );
  });
});

describe("StatusBadge", () => {
  it("names a step that never ran", () => {
    expect(mount(StatusBadge, { props: { status: "not_run" } }).text()).toBe("未执行");
  });
});
