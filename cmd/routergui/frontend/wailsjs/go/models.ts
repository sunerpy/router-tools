export namespace app {
	
	export class BackupCreateReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
	    output_dir: string;
	    manifest?: backup.Manifest;
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
	export class DeepResult {
	    status: string;
	    accepted_writes: number;
	    transitions?: BootTransition[];
	    final_ssh_verified: boolean;
	    soft_verified: boolean;
	
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
	
	export class DoctorReport {
	    schema_version: number;
	    operation: string;
	    status: string;
	    host: string;
	    device: device.Observation;
	    profile: device.Profile;
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
	
	    static createFrom(source: any = {}) {
	        return new RebootResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.old_boot_id = source["old_boot_id"];
	        this.new_boot_id = source["new_boot_id"];
	        this.ssh_verified = source["ssh_verified"];
	        this.soft_verified = source["soft_verified"];
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
	
	    static createFrom(source: any = {}) {
	        return new SoftResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outcome = source["outcome"];
	        this.verified = source["verified"];
	        this.reboot_verified = source["reboot_verified"];
	        this.warnings = source["warnings"];
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

export namespace device {
	
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
	export class Profile {
	    id: string;
	    vendor: string;
	    model: string;
	    hardware: string;
	    firmware: string;
	    capabilities: string[];
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.vendor = source["vendor"];
	        this.model = source["model"];
	        this.hardware = source["hardware"];
	        this.firmware = source["firmware"];
	        this.capabilities = source["capabilities"];
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

