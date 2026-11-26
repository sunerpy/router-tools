import { nextTick } from "vue";
import { useOperations } from "./useOperations";
import type { RouterAPI } from "../services/api";

function makeAPI(): RouterAPI {
  return {
    Doctor: vi.fn().mockResolvedValue({ status: "succeeded" }),
    Verify: vi.fn().mockResolvedValue({ status: "succeeded", ssh_verified: true }),
    EnableSSH: vi.fn().mockResolvedValue({ status: "succeeded", ssh_verified: true }),
    PersistSoft: vi.fn().mockResolvedValue({ status: "succeeded" }),
    PersistRemove: vi.fn().mockResolvedValue({ status: "succeeded" }),
    RebootVerify: vi.fn().mockResolvedValue({ status: "succeeded" }),
    BackupCreate: vi.fn().mockResolvedValue({ status: "succeeded" }),
    BackupVerify: vi.fn().mockResolvedValue({ status: "succeeded", valid: true }),
    BackupVerifyOnline: vi.fn().mockResolvedValue({ status: "succeeded", valid: true }),
    DeepPersist: vi.fn().mockResolvedValue({ status: "succeeded" }),
    GuidePlan: vi.fn(),
    GuideApply: vi.fn(),
    AcknowledgeRecovery: vi.fn(),
    CredentialStatus: vi.fn(),
    ForgetCredential: vi.fn(),
    ForgetAllCredentials: vi.fn(),
    Catalog: vi.fn().mockResolvedValue([]),
    Cancel: vi.fn().mockResolvedValue(true),
    ListReports: vi.fn().mockResolvedValue([]),
    ResolveHostKey: vi.fn(),
    OnProgress: vi.fn().mockReturnValue(() => {}),
    OnHostKey: vi.fn().mockReturnValue(() => {}),
  };
}

describe("useOperations", () => {
  it("runs an operation, clears secrets, and stores the report", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    store.target.host = "192.168.31.2";
    store.adminPassword = "admin-secret";
    store.rootPassword = "root-secret";
    store.withAdminAuth = true;
    store.withRootVerify = true;

    await store.run("Doctor");

    expect(api.Doctor).toHaveBeenCalledOnce();
    const request = vi.mocked(api.Doctor).mock.calls[0][0];
    expect(request.Target.host).toBe("192.168.31.2");
    expect(request.AdminPassword).toBe("admin-secret");
    expect(request.RootPassword).toBe("root-secret");
    expect(store.adminPassword).toBe("");
    expect(store.rootPassword).toBe("");
    expect(store.report).toEqual({ status: "succeeded" });
    expect(localStorage.getItem("router.preferences") || "").not.toContain("secret");
  });

  it("sends ports as integers even after the inputs emit strings", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    // Number inputs emit their raw string value; Go rejects strings for int fields.
    (store.target as any).web_port = "8080";
    (store.target as any).ssh_port = "1.5";
    (store.target as any).telnet_port = "";

    await store.run("Doctor");

    const request = vi.mocked(api.Doctor).mock.calls[0][0];
    expect(request.Target).toEqual({
      host: "192.168.31.1",
      web_port: 8080,
      ssh_port: 0,
      telnet_port: 0,
    });
  });

  it("sends 0 for ports outside the TCP range so Go can still decode the request", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    // Larger than Go's int: decoding would fail before the application validates it.
    (store.target as any).web_port = "99999999999999999999";
    (store.target as any).ssh_port = "-22";
    (store.target as any).telnet_port = 65536;

    await store.run("Doctor");

    const request = vi.mocked(api.Doctor).mock.calls[0][0];
    expect(request.Target.web_port).toBe(0);
    expect(request.Target.ssh_port).toBe(0);
    expect(request.Target.telnet_port).toBe(0);
  });

  it("applies per-run overrides without changing the shared option", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    store.allowTelnet = false;

    await store.run("DeepPersist", { AllowTelnet: true });

    expect(vi.mocked(api.DeepPersist).mock.calls[0][0].AllowTelnet).toBe(true);
    expect(store.allowTelnet).toBe(false);
  });

  it("remembers the target each run was sent to", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    store.target.host = "192.168.31.9";
    (store.target as any).web_port = "8080";

    await store.run("Doctor");
    const id = store.operationID;
    store.target.host = "10.0.0.1";

    expect(store.targetFor(id)).toEqual({
      host: "192.168.31.9",
      web_port: 8080,
      ssh_port: 22,
      telnet_port: 23,
    });
    expect(store.targetFor("unknown-id")).toBeUndefined();
  });

  it("clears passwords and acknowledgements on request", () => {
    const store = useOperations(makeAPI());
    store.adminPassword = "admin-secret";
    store.rootPassword = "root-secret";
    store.ackDeep = "RD08-DEEP";
    store.confirmed = true;

    store.clearSecrets();

    expect([store.adminPassword, store.rootPassword, store.ackDeep, store.confirmed]).toEqual([
      "",
      "",
      "",
      false,
    ]);
  });

  it("refreshes history after a rejected call", async () => {
    const api = makeAPI();
    vi.mocked(api.DeepPersist).mockRejectedValue(new Error("INTERNAL_ERROR: write deep journal"));
    const store = useOperations(api);

    await store.run("DeepPersist");

    expect(store.status).toBe("failed");
    expect(api.ListReports).toHaveBeenCalledOnce();
  });

  it("prevents concurrent operations", async () => {
    const api = makeAPI();
    let resolve!: (value: unknown) => void;
    vi.mocked(api.Doctor).mockImplementation(
      () =>
        new Promise((resolvePromise) => {
          resolve = resolvePromise;
        }),
    );
    const store = useOperations(api);
    const first = store.run("Doctor");
    await nextTick();
    await store.run("Verify");
    expect(api.Verify).not.toHaveBeenCalled();
    resolve({ status: "succeeded" });
    await first;
  });

  it("cancels the current operation ID", async () => {
    const api = makeAPI();
    vi.mocked(api.Doctor).mockImplementation(() => new Promise(() => {}));
    const store = useOperations(api);
    void store.run("Doctor");
    await nextTick();
    await store.cancel();
    expect(api.Cancel).toHaveBeenCalledWith(store.operationID);
  });

  it("handles host-key confirmation through the API", async () => {
    const api = makeAPI();
    let listener!: (prompt: any) => void;
    vi.mocked(api.OnHostKey).mockImplementation((callback) => {
      listener = callback;
      return () => {};
    });
    const store = useOperations(api);
    const prompt = {
      prompt_id: "prompt-1",
      address: "router:22",
      algorithm: "ssh-ed25519",
      fingerprint: "SHA256:abc",
    };
    listener(prompt);
    expect(store.hostKeyPrompt).toEqual(prompt);
    await store.resolveHostKey(true);
    expect(api.ResolveHostKey).toHaveBeenCalledWith("prompt-1", true);
    expect(store.hostKeyPrompt).toBeNull();
  });

  it("shows failed, partial, and unknown reports resolved by the backend", async () => {
    for (const status of ["failed", "partial", "unknown"]) {
      const api = makeAPI();
      const result = {
        status,
        operation: "enable",
        error: { code: "MAC_CLEANUP_FAILED", message: "cleanup failed" },
      };
      vi.mocked(api.EnableSSH).mockResolvedValue(result);
      const store = useOperations(api);

      await store.run("EnableSSH");

      expect(store.report).toEqual(result);
      expect(store.status).toBe(status);
      expect(store.error).toBe("cleanup failed");
      expect(api.ListReports).toHaveBeenCalledOnce();
    }
  });

  it("marks a canceled report as canceled", async () => {
    const api = makeAPI();
    vi.mocked(api.Doctor).mockResolvedValue({
      status: "failed",
      error: { code: "CANCELED", message: "operation canceled" },
    });
    const store = useOperations(api);

    await store.run("Doctor");

    expect(store.status).toBe("canceled");
    expect(store.busy).toBe(false);
    expect(store.report).toMatchObject({ error: { code: "CANCELED" } });
  });

  it("keeps partial and unknown statuses when a canceled run may have written", async () => {
    for (const status of ["partial", "unknown"]) {
      const api = makeAPI();
      vi.mocked(api.EnableSSH).mockResolvedValue({
        status,
        error: { code: "CANCELED", message: "operation canceled" },
      });
      const store = useOperations(api);

      await store.run("EnableSSH");

      expect(store.status).toBe(status);
      expect(store.error).toBe("operation canceled");
    }
  });

  it("reports a rejected call that produced no report", async () => {
    const api = makeAPI();
    vi.mocked(api.Doctor).mockRejectedValue(
      new Error("INVALID_PASSWORD: password contains a control character"),
    );
    const store = useOperations(api);
    store.adminPassword = "bad";

    await store.run("Doctor");

    expect(store.status).toBe("failed");
    expect(store.report).toBeNull();
    expect(store.error).toContain("INVALID_PASSWORD");
    expect(store.adminPassword).toBe("");
  });

  it("keeps deep acknowledgement and backup inputs in the request", async () => {
    const api = makeAPI();
    const store = useOperations(api);
    store.rootPassword = "root";
    store.allowTelnet = true;
    store.ackDeep = "RD08-DEEP";
    store.backupDir = "/backup";
    store.runDir = "/run";
    await store.run("DeepPersist");
    const request = vi.mocked(api.DeepPersist).mock.calls[0][0];
    expect(request.AckDeep).toBe("RD08-DEEP");
    expect(request.BackupDir).toBe("/backup");
    expect(request.RunDir).toBe("/run");
    expect(store.ackDeep).toBe("");
  });
});
