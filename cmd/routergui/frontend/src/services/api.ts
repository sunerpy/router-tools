export interface Target {
  host: string;
  web_port: number;
  ssh_port: number;
  telnet_port: number;
}

export interface HostKeyPreferences {
  KnownHostsPath: string;
  PinSHA256: string;
  AcceptNew: boolean;
}

export interface OperationRequest {
  OperationID: string;
  Target: Target;
  ProfileID: string;
  AdminPassword: string;
  RootPassword: string;
  Transport: string;
  AllowTelnet: boolean;
  Confirmed: boolean;
  WithAdminAuth: boolean;
  WithRootVerify: boolean;
  OutputDir: string;
  BackupDir: string;
  RunDir: string;
  AckDeep: string;
  ToolVersion: string;
  HostKey: HostKeyPreferences;
  /** Fill passwords left empty from the encrypted vault; deep persistence ignores it. */
  UseSavedPassword: boolean;
  /** Save typed passwords after the router accepted them; deep persistence ignores it. */
  RememberPassword: boolean;
}

export interface ProgressPayload {
  operation_id: string;
  /** step names the guided step an event belongs to; absent outside a guided run. */
  event: { stage: string; status: string; operation: string; time: string; step?: string };
}

export interface HostKeyPrompt {
  prompt_id: string;
  operation_id: string;
  address: string;
  algorithm: string;
  fingerprint: string;
  /** "agent" when an agent started the operation. */
  source?: string;
}

/** Deep-persistence safety parameters of one profile (device.DeepPolicy). */
export interface DeepPolicy {
  acknowledgement: string;
  reboot_transitions: number;
}

/** One exact model+firmware profile (device.Profile). */
export interface Profile {
  id: string;
  model_id: string;
  vendor: string;
  model: string;
  hardware: string;
  firmware: string;
  capabilities: string[] | null;
  deep?: DeepPolicy;
}

/** A registered profile whose model matches an observation; explains failures, never selects. */
export interface ProfileMatch {
  profile: Profile;
  exact: boolean;
}

/** Registered firmware profiles of one model in registration order (device.CatalogEntry). */
export interface CatalogEntry {
  model_id: string;
  vendor: string;
  model: string;
  hardware: string;
  profiles: Profile[];
}

/** A redacted application error (device.Error). */
export interface ReportError {
  code: string;
  message: string;
  retryable?: boolean;
  stage?: string;
}

/** Read-only device evidence (device.Observation). */
export interface Observation {
  device_id: string;
  hardware?: string;
  display_name?: string;
  rom_version: string;
  new_encrypt_mode: number;
}

/** Guided-flow request (guiapi.GuideRequest). Confirmation, PlanDigest and Confirmed apply a plan. */
export interface GuideRequest {
  OperationID: string;
  Target: Target;
  ProfileID: string;
  Goals: GuideGoal[];
  AdminPassword: string;
  RootPassword: string;
  ResetRootPassword: boolean;
  Confirmation: GuideConfirmation | null;
  PlanDigest: string;
  Confirmed: boolean;
  ToolVersion: string;
  HostKey: HostKeyPreferences;
  UseSavedPassword: boolean;
  RememberPassword: boolean;
}

export type GuideGoal = "ssh" | "backup" | "soft" | "reboot";
export type GuideStepID = "enable_ssh" | "backup" | "soft_install" | "reboot_verify";
export type GuideAction =
  | "run"
  | "skip"
  | "decide_after"
  | "needs_input"
  | "blocked"
  | "unsupported"
  | "not_run";

/** One planned step (app.GuideStep); reason, effect and input codes are translated by the UI. */
export interface GuideStep {
  id: GuideStepID;
  action: GuideAction;
  reason: string;
  effects?: string[];
  inputs?: string[];
  maintenance?: string;
}

/** The device identity a confirmation binds (app.ConfirmedIdentity). */
export interface ConfirmedIdentity {
  device_id: string;
  hardware: string;
  display_name?: string;
  rom_version: string;
  profile_id: string;
}

export interface GuideConfirmedStep {
  id: GuideStepID;
  action: GuideAction;
  effects?: string[];
}

/** Sent back unchanged with plan_digest to apply a plan (app.GuideConfirmation). Holds no secret. */
export interface GuideConfirmation {
  schema_version: number;
  target: Target;
  identity: ConfirmedIdentity;
  goals: GuideGoal[];
  reset_root_password: boolean;
  steps: GuideConfirmedStep[];
}

/** Adapter stage evidence kept with the record (state.WriteEvidence); absent keys mean none. */
export interface WriteEvidence {
  write_started?: boolean;
  accepted_steps?: number;
  attempted_macs?: string[];
  uncleaned_macs?: string[];
  journal_path?: string;
}

/** The write-ahead record of a router's last change (state.Outcome). Unset times are absent. */
export interface OutcomeRecord {
  schema_version: number;
  operation: string;
  host?: string;
  status: "in_progress" | "succeeded" | "failed" | "partial" | "unknown";
  error_code?: string;
  device_write_possible: boolean;
  write_evidence?: WriteEvidence;
  started_at: string;
  finished_at?: string;
  acknowledged_at?: string;
}

/** A locally verified backup in the managed directory (app.ManagedBackup). */
export interface ManagedBackup {
  path: string;
  created_at: string;
  boot_id: string;
}

export interface GuideState {
  ssh?: string;
  telnet?: string;
  root_ssh: "verified" | "auth_failed" | "not_checked";
  soft?: string;
  backup?: ManagedBackup;
  recovery?: OutcomeRecord;
  recovery_blocked: boolean;
  locked: boolean;
  /** A usable saved password exists and will be filled in (only with UseSavedPassword). */
  saved_admin: boolean;
  saved_root: boolean;
}

/** A guided plan (app.GuidePlanReport); confirmation and plan_digest exist only when ready. */
export interface GuidePlanReport {
  schema_version: number;
  operation: "guide plan";
  status: string;
  host: string;
  device: Observation;
  profile: Profile;
  candidates?: ProfileMatch[];
  goals: GuideGoal[];
  state: GuideState;
  steps: GuideStep[];
  ready: boolean;
  changes: boolean;
  required_inputs?: string[];
  confirmation?: GuideConfirmation;
  plan_digest?: string;
  credentials?: CredentialNote[];
  error?: ReportError;
}

/** The outcome of one guided step with its use-case report (app.GuideStepResult). */
export interface GuideStepResult {
  id: GuideStepID;
  action: GuideAction;
  reason?: string;
  status: string;
  effects?: string[];
  maintenance?: string;
  enable?: unknown;
  backup?: unknown;
  soft?: unknown;
  reboot?: unknown;
  error?: ReportError;
}

export interface GuideApplyReport {
  schema_version: number;
  operation: "guide apply";
  status: string;
  host: string;
  device: Observation;
  profile: Profile;
  plan_digest: string;
  steps: GuideStepResult[];
  error?: ReportError;
}

export interface RecoveryReport {
  schema_version: number;
  operation: "recovery ack";
  status: string;
  host: string;
  acknowledged: boolean;
  last?: OutcomeRecord;
  error?: ReportError;
}

/** Read-only deep-persistence state of the device (app.DeepInspection). */
export interface DeepInspection {
  flags: "all_set" | "none" | "mixed";
  crash_partition: boolean;
  crash_readable: boolean;
}

export type DeepBlocked =
  | "recovery_review_required"
  | "journal_exists"
  | "flags_already_set"
  | "flags_mixed"
  | "crash_partition_unavailable"
  | "no_verified_backup";

/** What the deep-persistence page shows before a run (app.DeepStatusReport). */
export interface DeepStatusReport {
  schema_version: number;
  operation: "deep status";
  status: string;
  host: string;
  device: Observation;
  profile: Profile;
  candidates?: ProfileMatch[];
  journal_path?: string;
  journal_exists: boolean;
  backups: ManagedBackup[];
  /** The probed SSH port state; root verification and inspection run only when it is open. */
  ssh?: string;
  root_ssh: "verified" | "auth_failed" | "not_checked";
  inspection?: DeepInspection;
  recovery?: OutcomeRecord;
  blocked: DeepBlocked[];
  ready: boolean;
  credentials?: CredentialNote[];
  error?: ReportError;
}

/** What happened to a saved or remembered password (app.CredentialNote); never the value. */
export interface CredentialNote {
  kind: "web-admin" | "root";
  action: "used_saved" | "saved" | "save_failed" | "rejected" | "unavailable";
  error?: ReportError;
}

/** Metadata of one saved password (credentials.Entry). */
export interface CredentialEntry {
  kind: "web-admin" | "root";
  scope: string;
  host: string;
  port: number;
  saved_at: string;
  rejected_at?: string;
}

export interface CredentialReport {
  schema_version: number;
  operation: string;
  status: string;
  entries: CredentialEntry[];
  error?: ReportError;
}

/** Selects saved passwords to forget (guiapi.CredentialFilter); empty fields match every entry. */
export interface CredentialFilter {
  Kind: string;
  Scope: string;
  Host: string;
  Port: number;
}

export interface HistoryEntry {
  operation_id: string;
  operation: string;
  report: unknown;
  time: string;
  /** "agent" when an agent started the operation; absent for the window's own runs. */
  source?: string;
}

/** What the agent page sets (mcpserver.SettingsChange); the token changes only through AgentNewToken. */
export interface AgentSettingsChange {
  enabled: boolean;
  listen_host: string;
  port: number;
  allow_writes: boolean;
}

/** What actually listens, next to the stored settings (mcpserver.ListenerStatus). */
export interface AgentListenerStatus extends AgentSettingsChange {
  listening: boolean;
  /** The bound host:port while listening. */
  address?: string;
  token_set: boolean;
  token_created_at?: string;
  last_error?: string;
}

/** One agent operation (mcpserver.OperationSummary). */
export interface AgentOperation {
  operation_id: string;
  tool: string;
  host?: string;
  status: string;
  done: boolean;
  started: string;
  ended?: string;
  error_code?: string;
}

/** One secret-free audit record of an agent call (mcpserver.AuditRecord). */
export interface AgentActivity {
  time: string;
  kind: "call" | "finish";
  tool: string;
  operation_id?: string;
  host?: string;
  status?: string;
  error_code?: string;
  duration_ms: number;
  done: boolean;
}

/** A documented agent host (agentsetup.Agent). */
export interface AgentHost {
  id: string;
  name: string;
  http: boolean;
  skill_dir?: string;
  docs?: string;
}

/** The desktop's agent access (agentaccess.Status). */
export interface AgentStatus {
  listener: AgentListenerStatus;
  /** The endpoint local agents use. */
  url: string;
  local_addresses: string[] | null;
  operations: AgentOperation[] | null;
  activity: AgentActivity[] | null;
  agents: AgentHost[] | null;
  /** Where 安装 Skill writes for each host with a skills directory, resolved by Go. */
  skill_paths?: Record<string, string>;
}

/** AgentApply's result: a token created by enabling without one is returned once. */
export interface AgentApplyResult {
  status: AgentListenerStatus;
  token?: string;
}

/** One host's configuration for the desktop's endpoint (agentsetup.Setup). */
export interface AgentSetup {
  agent: string;
  transport: string;
  file?: string;
  format: string;
  snippet: string;
  command?: string;
  notes?: string[] | null;
}

/** Install the skill for a host (agent) or into a directory the user picked (dir). */
export interface SkillRequest {
  agent?: string;
  dir?: string;
  force?: boolean;
}

/** installed, unchanged or updated; differs (ask before force), refused or failed with a message. */
export interface SkillReport {
  status: "installed" | "unchanged" | "updated" | "differs" | "refused" | "failed";
  path: string;
  files?: string[] | null;
  message?: string;
}

export interface RouterAPI {
  Doctor(request: OperationRequest): Promise<any>;
  Verify(request: OperationRequest): Promise<any>;
  EnableSSH(request: OperationRequest): Promise<any>;
  PersistSoft(request: OperationRequest): Promise<any>;
  PersistRemove(request: OperationRequest): Promise<any>;
  RebootVerify(request: OperationRequest): Promise<any>;
  BackupCreate(request: OperationRequest): Promise<any>;
  BackupVerify(request: OperationRequest): Promise<any>;
  BackupVerifyOnline(request: OperationRequest): Promise<any>;
  DeepPersist(request: OperationRequest): Promise<any>;
  GuidePlan(request: GuideRequest): Promise<GuidePlanReport>;
  GuideApply(request: GuideRequest): Promise<GuideApplyReport>;
  AcknowledgeRecovery(request: OperationRequest): Promise<RecoveryReport>;
  DeepStatus(request: OperationRequest): Promise<DeepStatusReport>;
  CredentialStatus(): Promise<CredentialReport>;
  ForgetCredential(filter: CredentialFilter): Promise<CredentialReport>;
  ForgetAllCredentials(): Promise<CredentialReport>;
  Catalog(): Promise<CatalogEntry[]>;
  Cancel(operationID: string): Promise<boolean>;
  ListReports(): Promise<HistoryEntry[]>;
  /** Opens the system folder picker; resolves an empty string when canceled. */
  SelectDirectory(): Promise<string>;
  ResolveHostKey(promptID: string, accept: boolean): void;
  OnProgress(callback: (payload: ProgressPayload) => void): () => void;
  OnHostKey(callback: (prompt: HostKeyPrompt) => void): () => void;
  /** Go stopped waiting for a prompt (two minutes passed, or its operation was canceled or ended) and refused the key. */
  OnHostKeyExpired(callback: (promptID: string) => void): () => void;
  AgentStatus(): Promise<AgentStatus>;
  /** Rejects with the reason when the change cannot be applied; the listener then keeps its state. */
  AgentApply(change: AgentSettingsChange): Promise<AgentApplyResult>;
  /** Replaces the access token and resolves it once. */
  AgentNewToken(): Promise<string>;
  /** Rejects for hosts without a Streamable HTTP setup (WorkBuddy). */
  AgentSetup(agent: string): Promise<AgentSetup>;
  AgentInstallSkill(request: SkillRequest): Promise<SkillReport>;
  AgentCancel(operationID: string): Promise<boolean>;
  OnAgentStatus(callback: (status: AgentStatus) => void): () => void;
}

export type OperationName =
  | "Doctor"
  | "Verify"
  | "EnableSSH"
  | "PersistSoft"
  | "PersistRemove"
  | "RebootVerify"
  | "BackupCreate"
  | "BackupVerify"
  | "BackupVerifyOnline"
  | "DeepPersist";
