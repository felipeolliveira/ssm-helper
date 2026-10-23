export namespace env {
	
	export class Target {
	    name: string;
	    profile: string;
	    ssmTarget: string;
	    ssmRegion: string;
	    ssmDocument: string;
	    remoteHost: string;
	    remotePort: string;
	    localPort: string;
	    dnsHosts: string[];
	    openUrls: string[];
	    chromeProfile: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.profile = source["profile"];
	        this.ssmTarget = source["ssmTarget"];
	        this.ssmRegion = source["ssmRegion"];
	        this.ssmDocument = source["ssmDocument"];
	        this.remoteHost = source["remoteHost"];
	        this.remotePort = source["remotePort"];
	        this.localPort = source["localPort"];
	        this.dnsHosts = source["dnsHosts"];
	        this.openUrls = source["openUrls"];
	        this.chromeProfile = source["chromeProfile"];
	    }
	}
	export class Profile {
	    name: string;
	    ssoUrl: string;
	    ssoRegion: string;
	    accountId: string;
	    roleName: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ssoUrl = source["ssoUrl"];
	        this.ssoRegion = source["ssoRegion"];
	        this.accountId = source["accountId"];
	        this.roleName = source["roleName"];
	    }
	}
	export class Env {
	    path: string;
	    title: string;
	    profiles: Profile[];
	    targets: Target[];
	
	    static createFrom(source: any = {}) {
	        return new Env(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.title = source["title"];
	        this.profiles = this.convertValues(source["profiles"], Profile);
	        this.targets = this.convertValues(source["targets"], Target);
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

export namespace main {
	
	export class AppearanceState {
	    appearance: settings.Appearance;
	    builtins: settings.Palette[];
	
	    static createFrom(source: any = {}) {
	        return new AppearanceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appearance = this.convertValues(source["appearance"], settings.Appearance);
	        this.builtins = this.convertValues(source["builtins"], settings.Palette);
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
	export class EnvInfo {
	    name: string;
	    file: string;
	    activeTunnels: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.file = source["file"];
	        this.activeTunnels = source["activeTunnels"];
	    }
	}
	export class EnvList {
	    dir: string;
	    active: string;
	    envs: EnvInfo[];
	
	    static createFrom(source: any = {}) {
	        return new EnvList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.active = source["active"];
	        this.envs = this.convertValues(source["envs"], EnvInfo);
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
	export class EnvState {
	    name: string;
	    file: string;
	    path: string;
	    env?: env.Env;
	    problems: string[];
	    error: string;
	    errorDetail: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.file = source["file"];
	        this.path = source["path"];
	        this.env = this.convertValues(source["env"], env.Env);
	        this.problems = source["problems"];
	        this.error = source["error"];
	        this.errorDetail = source["errorDetail"];
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
	export class LogEntry {
	    time: string;
	    source: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.source = source["source"];
	        this.text = source["text"];
	    }
	}
	export class NewEnvRequest {
	    name: string;
	    copyCurrent: boolean;
	    renameCurrentTo: string;
	
	    static createFrom(source: any = {}) {
	        return new NewEnvRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.copyCurrent = source["copyCurrent"];
	        this.renameCurrentTo = source["renameCurrentTo"];
	    }
	}
	export class Preferences {
	    hideSensitive: boolean;
	    openChromeOnConnect: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hideSensitive = source["hideSensitive"];
	        this.openChromeOnConnect = source["openChromeOnConnect"];
	    }
	}
	export class ProfileStatus {
	    name: string;
	    loggedIn: boolean;
	    expiresAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.loggedIn = source["loggedIn"];
	        this.expiresAt = source["expiresAt"];
	    }
	}
	export class SaveResult {
	    problems: string[];
	    state: EnvState;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.problems = source["problems"];
	        this.state = this.convertValues(source["state"], EnvState);
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
	export class TunnelState {
	    env: string;
	    target: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new TunnelState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.env = source["env"];
	        this.target = source["target"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}

}

export namespace settings {
	
	export class Palette {
	    name: string;
	    colors: Record<string, string>;
	    builtIn: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Palette(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.colors = source["colors"];
	        this.builtIn = source["builtIn"];
	    }
	}
	export class Appearance {
	    palette: string;
	    palettes: Palette[];
	
	    static createFrom(source: any = {}) {
	        return new Appearance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.palette = source["palette"];
	        this.palettes = this.convertValues(source["palettes"], Palette);
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

