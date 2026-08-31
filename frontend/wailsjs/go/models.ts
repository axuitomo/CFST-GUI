export namespace app {
	
	export class HealthResult {
	    configPath: string;
	    online: boolean;
	    schemaVersion: string;
	    service: string;
	    version: string;
	    wailsTransport: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.online = source["online"];
	        this.schemaVersion = source["schemaVersion"];
	        this.service = source["service"];
	        this.version = source["version"];
	        this.wailsTransport = source["wailsTransport"];
	    }
	}

}

export namespace appcore {
	
	export class CommandResult {
	    code: string;
	    data: any;
	    message: string;
	    ok: boolean;
	    schema_version: string;
	    task_id?: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new CommandResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.data = source["data"];
	        this.message = source["message"];
	        this.ok = source["ok"];
	        this.schema_version = source["schema_version"];
	        this.task_id = source["task_id"];
	        this.warnings = source["warnings"];
	    }
	}

}

