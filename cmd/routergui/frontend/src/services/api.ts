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
  event: { stage: string; status: string; operation: string; time: string };
}

export interface HostKeyPrompt {
  prompt_id: string;
  operation_id: string;
  address: string;
  algorithm: string;
  fingerprint: string;
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
