export namespace app {
	
	export class CredentialNote {
	    kind: string;
	    action: string;
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new CredentialNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.action = source["action"];
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BackupCreateReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    output_dir: string;
	    manifest?: backup.Manifest;
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new BackupCreateReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.output_dir = source["output_dir"];
	        this.manifest = this.convertValues(source["manifest"], backup.Manifest);
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BackupVerifyReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    directory: string;
	    valid: boolean;
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new BackupVerifyReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.directory = source["directory"];
	        this.valid = source["valid"];
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BootTransition {
	    old_boot_id: string;
	    new_boot_id: string;
	
	    static createFrom(source: any = {}) {
	        return new BootTransition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.old_boot_id = source["old_boot_id"];
	        this.new_boot_id = source["new_boot_id"];
	    }
	}
	export class CleanupResult {
	    mac: string;
	    attempts: number;
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CleanupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mac = source["mac"];
	        this.attempts = source["attempts"];
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	}
	export class ConfirmedIdentity {
	    device_id: string;
	    hardware: string;
	    display_name?: string;
	    rom_version: string;
	    profile_id: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfirmedIdentity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.hardware = source["hardware"];
	        this.display_name = source["display_name"];
	        this.rom_version = source["rom_version"];
	        this.profile_id = source["profile_id"];
	    }
	}
	
	export class CredentialReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    entries: credentials.Entry[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new CredentialReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.entries = this.convertValues(source["entries"], credentials.Entry);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeepInspection {
	    flags: string;
	    crash_partition: boolean;
	    crash_readable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeepInspection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flags = source["flags"];
	        this.crash_partition = source["crash_partition"];
	        this.crash_readable = source["crash_readable"];
	    }
	}
	export class DeepResult {
	    status: string;
	    accepted_writes: number;
	    transitions?: BootTransition[];
	    final_ssh_verified: boolean;
	    soft_verified: boolean;
	    write_started: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.accepted_writes = source["accepted_writes"];
	        this.transitions = this.convertValues(source["transitions"], BootTransition);
	        this.final_ssh_verified = source["final_ssh_verified"];
	        this.soft_verified = source["soft_verified"];
	        this.write_started = source["write_started"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeepReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    result: DeepResult;
	    journal_path?: string;
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new DeepReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.result = this.convertValues(source["result"], DeepResult);
	        this.journal_path = source["journal_path"];
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ManagedBackup {
	    path: string;
	    // Go type: time
	    created_at: any;
	    boot_id: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedBackup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.boot_id = source["boot_id"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeepStatusReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    candidates?: device.ProfileMatch[];
	    journal_path?: string;
	    journal_exists: boolean;
	    backups: ManagedBackup[];
	    root_ssh: string;
	    inspection?: DeepInspection;
	    recovery?: state.Outcome;
	    blocked: string[];
	    ready: boolean;
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new DeepStatusReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.candidates = this.convertValues(source["candidates"], device.ProfileMatch);
	        this.journal_path = source["journal_path"];
	        this.journal_exists = source["journal_exists"];
	        this.backups = this.convertValues(source["backups"], ManagedBackup);
	        this.root_ssh = source["root_ssh"];
	        this.inspection = this.convertValues(source["inspection"], DeepInspection);
	        this.recovery = this.convertValues(source["recovery"], state.Outcome);
	        this.blocked = source["blocked"];
	        this.ready = source["ready"];
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DoctorReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    candidates?: device.ProfileMatch[];
	    compatible: boolean;
	    ssh: string;
	    telnet: string;
	    admin_auth: string;
	    root_verify: string;
	    transport?: string;
	    ssh_verified: boolean;
	    telnet_root_verified: boolean;
	    boot_id?: string;
	    next_steps?: string[];
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new DoctorReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.candidates = this.convertValues(source["candidates"], device.ProfileMatch);
	        this.compatible = source["compatible"];
	        this.ssh = source["ssh"];
	        this.telnet = source["telnet"];
	        this.admin_auth = source["admin_auth"];
	        this.root_verify = source["root_verify"];
	        this.transport = source["transport"];
	        this.ssh_verified = source["ssh_verified"];
	        this.telnet_root_verified = source["telnet_root_verified"];
	        this.boot_id = source["boot_id"];
	        this.next_steps = source["next_steps"];
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EnableReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    outcome?: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    accepted_steps: number;
	    ssh_verified: boolean;
	    attempted_macs?: string[];
	    cleanup?: CleanupResult[];
	    warnings?: string[];
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new EnableReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.outcome = source["outcome"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.accepted_steps = source["accepted_steps"];
	        this.ssh_verified = source["ssh_verified"];
	        this.attempted_macs = source["attempted_macs"];
	        this.cleanup = this.convertValues(source["cleanup"], CleanupResult);
	        this.warnings = source["warnings"];
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RebootResult {
	    old_boot_id: string;
	    new_boot_id: string;
	    ssh_verified: boolean;
	    soft_verified: boolean;
	    write_started: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RebootResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.old_boot_id = source["old_boot_id"];
	        this.new_boot_id = source["new_boot_id"];
	        this.ssh_verified = source["ssh_verified"];
	        this.soft_verified = source["soft_verified"];
	        this.write_started = source["write_started"];
	    }
	}
	export class RebootReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    result: RebootResult;
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new RebootReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.result = this.convertValues(source["result"], RebootResult);
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SoftResult {
	    outcome: string;
	    verified: boolean;
	    reboot_verified: boolean;
	    warnings?: string[];
	    write_started: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SoftResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outcome = source["outcome"];
	        this.verified = source["verified"];
	        this.reboot_verified = source["reboot_verified"];
	        this.warnings = source["warnings"];
	        this.write_started = source["write_started"];
	    }
	}
	export class SoftReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    result: SoftResult;
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new SoftReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.result = this.convertValues(source["result"], SoftResult);
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GuideStepResult {
	    id: string;
	    action: string;
	    reason?: string;
	    status: string;
	    effects?: string[];
	    maintenance?: string;
	    enable?: EnableReport;
	    backup?: BackupCreateReport;
	    soft?: SoftReport;
	    reboot?: RebootReport;
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new GuideStepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.action = source["action"];
	        this.reason = source["reason"];
	        this.status = source["status"];
	        this.effects = source["effects"];
	        this.maintenance = source["maintenance"];
	        this.enable = this.convertValues(source["enable"], EnableReport);
	        this.backup = this.convertValues(source["backup"], BackupCreateReport);
	        this.soft = this.convertValues(source["soft"], SoftReport);
	        this.reboot = this.convertValues(source["reboot"], RebootReport);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GuideApplyReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    plan_digest: string;
	    steps: GuideStepResult[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new GuideApplyReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.plan_digest = source["plan_digest"];
	        this.steps = this.convertValues(source["steps"], GuideStepResult);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GuideConfirmedStep {
	    id: string;
	    action: string;
	    effects?: string[];
	
	    static createFrom(source: any = {}) {
	        return new GuideConfirmedStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.action = source["action"];
	        this.effects = source["effects"];
	    }
	}
	export class GuideConfirmation {
	    schema_version: number;
	    target: device.Target;
	    identity: ConfirmedIdentity;
	    goals: string[];
	    reset_root_password: boolean;
	    steps: GuideConfirmedStep[];
	
	    static createFrom(source: any = {}) {
	        return new GuideConfirmation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.target = this.convertValues(source["target"], device.Target);
	        this.identity = this.convertValues(source["identity"], ConfirmedIdentity);
	        this.goals = source["goals"];
	        this.reset_root_password = source["reset_root_password"];
	        this.steps = this.convertValues(source["steps"], GuideConfirmedStep);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class GuideStep {
	    id: string;
	    action: string;
	    reason: string;
	    effects?: string[];
	    inputs?: string[];
	    maintenance?: string;
	
	    static createFrom(source: any = {}) {
	        return new GuideStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.action = source["action"];
	        this.reason = source["reason"];
	        this.effects = source["effects"];
	        this.inputs = source["inputs"];
	        this.maintenance = source["maintenance"];
	    }
	}
	export class GuideState {
	    ssh?: string;
	    telnet?: string;
	    root_ssh: string;
	    soft?: string;
	    backup?: ManagedBackup;
	    recovery?: state.Outcome;
	    recovery_blocked: boolean;
	    locked: boolean;
	    saved_admin: boolean;
	    saved_root: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GuideState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssh = source["ssh"];
	        this.telnet = source["telnet"];
	        this.root_ssh = source["root_ssh"];
	        this.soft = source["soft"];
	        this.backup = this.convertValues(source["backup"], ManagedBackup);
	        this.recovery = this.convertValues(source["recovery"], state.Outcome);
	        this.recovery_blocked = source["recovery_blocked"];
	        this.locked = source["locked"];
	        this.saved_admin = source["saved_admin"];
	        this.saved_root = source["saved_root"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GuidePlanReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    candidates?: device.ProfileMatch[];
	    goals: string[];
	    state: GuideState;
	    steps: GuideStep[];
	    ready: boolean;
	    changes: boolean;
	    required_inputs?: string[];
	    confirmation?: GuideConfirmation;
	    plan_digest?: string;
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new GuidePlanReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.candidates = this.convertValues(source["candidates"], device.ProfileMatch);
	        this.goals = source["goals"];
	        this.state = this.convertValues(source["state"], GuideState);
	        this.steps = this.convertValues(source["steps"], GuideStep);
	        this.ready = source["ready"];
	        this.changes = source["changes"];
	        this.required_inputs = source["required_inputs"];
	        this.confirmation = this.convertValues(source["confirmation"], GuideConfirmation);
	        this.plan_digest = source["plan_digest"];
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	export class RecoveryReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    acknowledged: boolean;
	    last?: state.Outcome;
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new RecoveryReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.acknowledged = source["acknowledged"];
	        this.last = this.convertValues(source["last"], state.Outcome);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class VerifyReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    transport: string;
	    root_verified: boolean;
	    ssh_verified: boolean;
	    telnet_root_verified: boolean;
	    boot_id?: string;
	    credentials?: CredentialNote[];
	    error?: device.Error;
	
	    static createFrom(source: any = {}) {
	        return new VerifyReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.host = source["host"];
	        this.device = this.convertValues(source["device"], device.Observation);
	        this.profile = this.convertValues(source["profile"], device.Profile);
	        this.transport = source["transport"];
	        this.root_verified = source["root_verified"];
	        this.ssh_verified = source["ssh_verified"];
	        this.telnet_root_verified = source["telnet_root_verified"];
	        this.boot_id = source["boot_id"];
	        this.credentials = this.convertValues(source["credentials"], CredentialNote);
	        this.error = this.convertValues(source["error"], device.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace backup {
	
	export class DeviceIdentity {
	    device_id: string;
	    hardware: string;
	    rom_version: string;
	    profile: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceIdentity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.hardware = source["hardware"];
	        this.rom_version = source["rom_version"];
	        this.profile = source["profile"];
	    }
	}
	export class Partition {
	    index: number;
	    name: string;
	    size: number;
	    erase_size: number;
	    source: string;
	    file: string;
	    sha256: string;
	
	    static createFrom(source: any = {}) {
	        return new Partition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.erase_size = source["erase_size"];
	        this.source = source["source"];
	        this.file = source["file"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class Manifest {
	    schema_version: number;
	    tool_version: string;
	    // Go type: time
	    created_at: any;
	    complete: boolean;
	    device: DeviceIdentity;
	    boot_id: string;
	    mtd_text_file: string;
	    partitions: Partition[];
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.tool_version = source["tool_version"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.complete = source["complete"];
	        this.device = this.convertValues(source["device"], DeviceIdentity);
	        this.boot_id = source["boot_id"];
	        this.mtd_text_file = source["mtd_text_file"];
	        this.partitions = this.convertValues(source["partitions"], Partition);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace credentials {
	
	export class Entry {
	    kind: string;
	    scope: string;
	    host: string;
	    port: number;
	    // Go type: time
	    saved_at: any;
	    // Go type: time
	    rejected_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.scope = source["scope"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.saved_at = this.convertValues(source["saved_at"], null);
	        this.rejected_at = this.convertValues(source["rejected_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace device {
	
	export class DeepPolicy {
	    acknowledgement: string;
	    reboot_transitions: number;
	
	    static createFrom(source: any = {}) {
	        return new DeepPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.acknowledgement = source["acknowledgement"];
	        this.reboot_transitions = source["reboot_transitions"];
	    }
	}
	export class Profile {
	    id: string;
	    model_id: string;
	    vendor: string;
	    model: string;
	    hardware: string;
	    firmware: string;
	    capabilities: string[];
	    deep?: DeepPolicy;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.model_id = source["model_id"];
	        this.vendor = source["vendor"];
	        this.model = source["model"];
	        this.hardware = source["hardware"];
	        this.firmware = source["firmware"];
	        this.capabilities = source["capabilities"];
	        this.deep = this.convertValues(source["deep"], DeepPolicy);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CatalogEntry {
	    model_id: string;
	    vendor: string;
	    model: string;
	    hardware: string;
	    profiles: Profile[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model_id = source["model_id"];
	        this.vendor = source["vendor"];
	        this.model = source["model"];
	        this.hardware = source["hardware"];
	        this.profiles = this.convertValues(source["profiles"], Profile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Error {
	    code: string;
	    message: string;
	    retryable: boolean;
	    stage?: string;
	
	    static createFrom(source: any = {}) {
	        return new Error(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.retryable = source["retryable"];
	        this.stage = source["stage"];
	    }
	}
	export class Observation {
	    device_id: string;
	    hardware?: string;
	    display_name?: string;
	    rom_version: string;
	    new_encrypt_mode: number;
	
	    static createFrom(source: any = {}) {
	        return new Observation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.hardware = source["hardware"];
	        this.display_name = source["display_name"];
	        this.rom_version = source["rom_version"];
	        this.new_encrypt_mode = source["new_encrypt_mode"];
	    }
	}
	
	export class ProfileMatch {
	    profile: Profile;
	    exact: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], Profile);
	        this.exact = source["exact"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Target {
	    host: string;
	    web_port: number;
	    ssh_port: number;
	    telnet_port: number;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.web_port = source["web_port"];
	        this.ssh_port = source["ssh_port"];
	        this.telnet_port = source["telnet_port"];
	    }
	}

}

export namespace guiapi {
	
	export class CredentialFilter {
	    Kind: string;
	    Scope: string;
	    Host: string;
	    Port: number;
	
	    static createFrom(source: any = {}) {
	        return new CredentialFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Kind = source["Kind"];
	        this.Scope = source["Scope"];
	        this.Host = source["Host"];
	        this.Port = source["Port"];
	    }
	}
	export class HostKeyRequest {
	    KnownHostsPath: string;
	    PinSHA256: string;
	    AcceptNew: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HostKeyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.KnownHostsPath = source["KnownHostsPath"];
	        this.PinSHA256 = source["PinSHA256"];
	        this.AcceptNew = source["AcceptNew"];
	    }
	}
	export class GuideRequest {
	    OperationID: string;
	    Target: device.Target;
	    ProfileID: string;
	    Goals: string[];
	    AdminPassword: string;
	    RootPassword: string;
	    ResetRootPassword: boolean;
	    Confirmation?: app.GuideConfirmation;
	    PlanDigest: string;
	    Confirmed: boolean;
	    ToolVersion: string;
	    HostKey: HostKeyRequest;
	    UseSavedPassword: boolean;
	    RememberPassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GuideRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.OperationID = source["OperationID"];
	        this.Target = this.convertValues(source["Target"], device.Target);
	        this.ProfileID = source["ProfileID"];
	        this.Goals = source["Goals"];
	        this.AdminPassword = source["AdminPassword"];
	        this.RootPassword = source["RootPassword"];
	        this.ResetRootPassword = source["ResetRootPassword"];
	        this.Confirmation = this.convertValues(source["Confirmation"], app.GuideConfirmation);
	        this.PlanDigest = source["PlanDigest"];
	        this.Confirmed = source["Confirmed"];
	        this.ToolVersion = source["ToolVersion"];
	        this.HostKey = this.convertValues(source["HostKey"], HostKeyRequest);
	        this.UseSavedPassword = source["UseSavedPassword"];
	        this.RememberPassword = source["RememberPassword"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HistoryEntry {
	    operation_id: string;
	    operation: string;
	    report: any;
	    // Go type: time
	    time: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operation_id = source["operation_id"];
	        this.operation = source["operation"];
	        this.report = source["report"];
	        this.time = this.convertValues(source["time"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class OperationRequest {
	    OperationID: string;
	    Target: device.Target;
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
	    HostKey: HostKeyRequest;
	    UseSavedPassword: boolean;
	    RememberPassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OperationRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.OperationID = source["OperationID"];
	        this.Target = this.convertValues(source["Target"], device.Target);
	        this.ProfileID = source["ProfileID"];
	        this.AdminPassword = source["AdminPassword"];
	        this.RootPassword = source["RootPassword"];
	        this.Transport = source["Transport"];
	        this.AllowTelnet = source["AllowTelnet"];
	        this.Confirmed = source["Confirmed"];
	        this.WithAdminAuth = source["WithAdminAuth"];
	        this.WithRootVerify = source["WithRootVerify"];
	        this.OutputDir = source["OutputDir"];
	        this.BackupDir = source["BackupDir"];
	        this.RunDir = source["RunDir"];
	        this.AckDeep = source["AckDeep"];
	        this.ToolVersion = source["ToolVersion"];
	        this.HostKey = this.convertValues(source["HostKey"], HostKeyRequest);
	        this.UseSavedPassword = source["UseSavedPassword"];
	        this.RememberPassword = source["RememberPassword"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace state {
	
	export class WriteEvidence {
	    write_started: boolean;
	    accepted_steps: number;
	    attempted_macs?: string[];
	    uncleaned_macs?: string[];
	    journal_path?: string;
	
	    static createFrom(source: any = {}) {
	        return new WriteEvidence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.write_started = source["write_started"];
	        this.accepted_steps = source["accepted_steps"];
	        this.attempted_macs = source["attempted_macs"];
	        this.uncleaned_macs = source["uncleaned_macs"];
	        this.journal_path = source["journal_path"];
	    }
	}
	export class Outcome {
	    schema_version: number;
	    operation: string;
	    host?: string;
	    status: string;
	    error_code?: string;
	    device_write_possible: boolean;
	    write_evidence: WriteEvidence;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    finished_at: any;
	    // Go type: time
	    acknowledged_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Outcome(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.operation = source["operation"];
	        this.host = source["host"];
	        this.status = source["status"];
	        this.error_code = source["error_code"];
	        this.device_write_possible = source["device_write_possible"];
	        this.write_evidence = this.convertValues(source["write_evidence"], WriteEvidence);
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.finished_at = this.convertValues(source["finished_at"], null);
	        this.acknowledged_at = this.convertValues(source["acknowledged_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

