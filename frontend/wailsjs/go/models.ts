export namespace main {

	export class StandaloneStartOptions {
	    allow_delete: boolean;
	    port: number;
	    auto_open_browser: boolean;

	    static createFrom(source: any = {}) {
	        return new StandaloneStartOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allow_delete = source["allow_delete"];
	        this.port = source["port"];
	        this.auto_open_browser = source["auto_open_browser"];
	    }
	}
	export class StartOptions {
	    thread: number;
	    allow_delete: boolean;
	    port: number;
	    auto_open_browser: boolean;

	    static createFrom(source: any = {}) {
	        return new StartOptions(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.thread = source["thread"];
	        this.allow_delete = source["allow_delete"];
	        this.port = source["port"];
	        this.auto_open_browser = source["auto_open_browser"];
	    }
	}
	export class CertificatePaths {
	    server_public_path: string;
	    client_private_path: string;

	    static createFrom(source: any = {}) {
	        return new CertificatePaths(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_public_path = source["server_public_path"];
	        this.client_private_path = source["client_private_path"];
	    }
	}
	export class PeriodicConfig {
	    uid: number;
	    periodic_port: string;
	    rsa_mode: string;
	    client_name: string;
	    client_token: string;
	    func_prefix: string;
	    env: Record<string, string>;
	    env_text: string;
	    created_at: number;
	    updated_at: number;

	    static createFrom(source: any = {}) {
	        return new PeriodicConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.periodic_port = source["periodic_port"];
	        this.rsa_mode = source["rsa_mode"];
	        this.client_name = source["client_name"];
	        this.client_token = source["client_token"];
	        this.func_prefix = source["func_prefix"];
	        this.env = source["env"];
	        this.env_text = source["env_text"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class UserInfo {
	    name: string;
	    nick_name: string;
	    avatar_url: string;

	    static createFrom(source: any = {}) {
	        return new UserInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.nick_name = source["nick_name"];
	        this.avatar_url = source["avatar_url"];
	    }
	}
	export class AppStatus {
	    api_base_url: string;
	    logged_in: boolean;
	    user_name: string;
	    user_info: UserInfo;
	    config?: PeriodicConfig;
	    certificate?: CertificatePaths;
	    root_dir: string;
	    start_options: StartOptions;
	    running: boolean;
	    web_running: boolean;
	    web_url: string;
	    standalone_root_dir: string;
	    standalone_start_options: StandaloneStartOptions;
	    standalone_web_running: boolean;
	    standalone_web_url: string;
	    last_error: string;
	    logs: string[];
	    binary_target: string;

	    static createFrom(source: any = {}) {
	        return new AppStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_base_url = source["api_base_url"];
	        this.logged_in = source["logged_in"];
	        this.user_name = source["user_name"];
	        this.user_info = this.convertValues(source["user_info"], UserInfo);
	        this.config = this.convertValues(source["config"], PeriodicConfig);
	        this.certificate = this.convertValues(source["certificate"], CertificatePaths);
	        this.root_dir = source["root_dir"];
	        this.start_options = this.convertValues(source["start_options"], StartOptions);
	        this.running = source["running"];
	        this.web_running = source["web_running"];
	        this.web_url = source["web_url"];
	        this.standalone_root_dir = source["standalone_root_dir"];
	        this.standalone_start_options = this.convertValues(source["standalone_start_options"], StandaloneStartOptions);
	        this.standalone_web_running = source["standalone_web_running"];
	        this.standalone_web_url = source["standalone_web_url"];
	        this.last_error = source["last_error"];
	        this.logs = source["logs"];
	        this.binary_target = source["binary_target"];
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

