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

export interface HistoryEntry {
  operation_id: string;
  operation: string;
  report: unknown;
  time: string;
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
  Catalog(): Promise<CatalogEntry[]>;
  Cancel(operationID: string): Promise<boolean>;
  ListReports(): Promise<HistoryEntry[]>;
  ResolveHostKey(promptID: string, accept: boolean): void;
  OnProgress(callback: (payload: ProgressPayload) => void): () => void;
  OnHostKey(callback: (prompt: HostKeyPrompt) => void): () => void;
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
