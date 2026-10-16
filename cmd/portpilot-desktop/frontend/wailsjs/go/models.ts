export namespace desktop {
	
	export class ActivePortDTO {
	    port: number;
	    protocol: string;
	    pid: number;
	    process?: string;
	    service?: string;
	    verified: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ActivePortDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.pid = source["pid"];
	        this.process = source["process"];
	        this.service = source["service"];
	        this.verified = source["verified"];
	    }
	}
	export class KillResult {
	    port: number;
	    pid: number;
	    status: string;
	    service?: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new KillResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.pid = source["pid"];
	        this.status = source["status"];
	        this.service = source["service"];
	        this.message = source["message"];
	    }
	}
	export class ListPortsResult {
	    ports: ActivePortDTO[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ListPortsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ports = this.convertValues(source["ports"], ActivePortDTO);
	        this.error = source["error"];
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
	export class PortCheckResult {
	    port: number;
	    service?: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PortCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.service = source["service"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class PortOwner {
	    port: number;
	    pid: number;
	    process: string;
	    verified: boolean;
	    service?: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PortOwner(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.pid = source["pid"];
	        this.process = source["process"];
	        this.verified = source["verified"];
	        this.service = source["service"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}

}

