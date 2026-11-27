import { createAPI } from "./wails";
import type { GuideRequest } from "./api";

function guideRequest(overrides: Partial<GuideRequest> = {}): GuideRequest {
  return {
    OperationID: "guide-op",
    Target: { host: "192.168.31.1", web_port: 80, ssh_port: 22, telnet_port: 23 },
    ProfileID: "auto",
    Goals: ["ssh", "backup", "soft"],
    AdminPassword: "",
    RootPassword: "",
    ResetRootPassword: false,
    Confirmation: null,
    PlanDigest: "",
    Confirmed: false,
    HostKey: { KnownHostsPath: "", PinSHA256: "", AcceptNew: false },
    UseSavedPassword: false,
    RememberPassword: false,
    ...overrides,
  };
}

describe("preview saved passwords", () => {
  afterEach(() => {
    delete window.__ROUTER_PREVIEW_SCENARIO__;
  });

  it("starts empty and remembers only after a successful run that asked to", async () => {
    const api = createAPI();
    expect((await api.CredentialStatus()).entries).toEqual([]);

    const plan = await api.GuidePlan(guideRequest());
    await api.GuideApply(
      guideRequest({
        Confirmation: plan.confirmation!,
        PlanDigest: plan.plan_digest!,
        Confirmed: true,
        AdminPassword: "admin-secret",
        RootPassword: "root-secret",
        RememberPassword: true,
      }),
    );
    const status = await api.CredentialStatus();
    expect(
      status.entries.map((entry) => `${entry.kind}:${entry.host}:${entry.port}`).sort(),
    ).toEqual(["root:192.168.31.1:22", "web-admin:192.168.31.1:80"]);
    expect(JSON.stringify(status)).not.toMatch(/admin-secret|root-secret/);
  });

  it("counts a saved admin password as provided in the next plan", async () => {
    const api = createAPI();
    const first = await api.GuidePlan(guideRequest());
    await api.GuideApply(
      guideRequest({
        Confirmation: first.confirmation!,
        PlanDigest: first.plan_digest!,
        Confirmed: true,
        AdminPassword: "admin-secret",
        RootPassword: "root-secret",
        RememberPassword: true,
      }),
    );
    const plan = await api.GuidePlan(guideRequest({ UseSavedPassword: true }));
    expect(plan.state.saved_admin).toBe(true);
    expect(plan.required_inputs).toEqual(["new_root_password"]);
  });

  it("forgets by filter and forgets all", async () => {
    const api = createAPI();
    const plan = await api.GuidePlan(guideRequest());
    await api.GuideApply(
      guideRequest({
        Confirmation: plan.confirmation!,
        PlanDigest: plan.plan_digest!,
        Confirmed: true,
        AdminPassword: "admin-secret",
        RootPassword: "root-secret",
        RememberPassword: true,
      }),
    );
    const refused = await api.ForgetCredential({ Kind: "", Scope: "", Host: "", Port: 0 });
    expect(refused.status).toBe("failed");
    expect(refused.error?.code).toBe("INVALID_ARGUMENT");
    expect((await api.CredentialStatus()).entries).toHaveLength(2);
    const forgotten = await api.ForgetCredential({ Kind: "root", Scope: "", Host: "", Port: 0 });
    expect(forgotten.entries.map((entry) => entry.kind)).toEqual(["root"]);
    expect((await api.CredentialStatus()).entries.map((entry) => entry.kind)).toEqual([
      "web-admin",
    ]);
    await api.ForgetAllCredentials();
    expect((await api.CredentialStatus()).entries).toEqual([]);
  });
});

describe("preview guided run without passwords", () => {
  it("fails like the application when no password is typed or saved", async () => {
    const api = createAPI();
    const plan = await api.GuidePlan(guideRequest());
    const report = await api.GuideApply(
      guideRequest({
        Confirmation: plan.confirmation!,
        PlanDigest: plan.plan_digest!,
        Confirmed: true,
        RememberPassword: true,
      }),
    );
    expect(report.status).toBe("failed");
    expect(report.error?.code).toBe("INVALID_PASSWORD");
    expect((await api.CredentialStatus()).entries).toEqual([]);
  });
});
