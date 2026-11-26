import { createAPI } from "./wails";
import type { OperationRequest, ProgressPayload } from "./api";

function makeRequest(): OperationRequest {
  return {
    OperationID: "preview-op",
    Target: { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 },
    ProfileID: "auto",
    AdminPassword: "",
    RootPassword: "",
    Transport: "ssh",
    AllowTelnet: false,
    Confirmed: false,
    WithAdminAuth: false,
    WithRootVerify: false,
    OutputDir: "",
    BackupDir: "",
    RunDir: "",
    AckDeep: "",
    ToolVersion: "dev",
    UseSavedPassword: false,
    RememberPassword: false,
    HostKey: { KnownHostsPath: "", PinSHA256: "", AcceptNew: false },
  };
}

describe("preview API", () => {
  afterEach(() => {
    delete window.__ROUTER_PREVIEW_SCENARIO__;
  });

  it("resolves a failed report the way the desktop bindings do", async () => {
    window.__ROUTER_PREVIEW_SCENARIO__ = "failed";
    const api = createAPI();
    const events: ProgressPayload["event"][] = [];
    api.OnProgress((payload) => events.push(payload.event));

    const report = await api.Doctor(makeRequest());

    expect(report.status).toBe("failed");
    expect(report.error.code).toBe("WEB_UNREACHABLE");
    expect(report.compatible).toBe(false);
    expect(report.ssh_verified).toBe(false);
    expect(events.at(-1)).toMatchObject({ stage: "failed", status: "failed" });
    expect((await api.ListReports())[0].report).toBe(report);
  });

  it("returns a preflight report with identity and port evidence", async () => {
    const api = createAPI();
    const report = await api.Doctor({ ...makeRequest(), WithAdminAuth: true });
    expect(report.device).toMatchObject({ hardware: "RD08", rom_version: "1.1.96" });
    expect(report.profile.id).toBe("xiaomi-rd08-1.1.96");
    expect(report.profile.capabilities).toContain("deep_persist");
    expect(report).toMatchObject({ ssh: "open", telnet: "closed", admin_auth: "verified" });
    expect(report.ssh_verified).toBe(false);
  });

  it("lists the registered device catalog like the Go Catalog binding", async () => {
    const api = createAPI();
    const catalog = await api.Catalog();
    expect(catalog).toHaveLength(1);
    expect(catalog[0]).toMatchObject({
      model_id: "xiaomi-rd08",
      vendor: "Xiaomi",
      model: "BE6500 Pro",
      hardware: "RD08",
    });
    expect(catalog[0].profiles.map((profile) => profile.id)).toEqual(["xiaomi-rd08-1.1.96"]);
    expect(catalog[0].profiles[0].deep).toEqual({
      acknowledgement: "RD08-DEEP",
      reboot_transitions: 3,
    });
    const report = await api.Doctor(makeRequest());
    expect(report.profile).toEqual(catalog[0].profiles[0]);
  });

  it("keeps the deep journal in the managed directory that the deep status shows", async () => {
    const api = createAPI();
    const status = await api.DeepStatus(makeRequest());
    const report = await api.DeepPersist({ ...makeRequest(), RunDir: "" });
    expect(report.journal_path).toBe(status.journal_path);
  });

  it("keeps the success scenario as the default", async () => {
    const api = createAPI();
    const report = await api.Doctor(makeRequest());
    expect(report.status).toBe("succeeded");
    expect(report.error).toBeUndefined();
  });
});
