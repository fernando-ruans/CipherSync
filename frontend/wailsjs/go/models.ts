export namespace main {
	
	export class ItemRef {
	    id: string;
	    title: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new ItemRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.score = source["score"];
	    }
	}
	export class DuplicateGroup {
	    password: string;
	    items: ItemRef[];
	
	    static createFrom(source: any = {}) {
	        return new DuplicateGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.password = source["password"];
	        this.items = this.convertValues(source["items"], ItemRef);
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
	export class FieldMapping {
	    column: number;
	    field: string;
	
	    static createFrom(source: any = {}) {
	        return new FieldMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.column = source["column"];
	        this.field = source["field"];
	    }
	}
	export class HealthReport {
	    totalItems: number;
	    totalPasswords: number;
	    totalScore: number;
	    weakCount: number;
	    duplicateCount: number;
	    oldCount: number;
	    missing2FA: number;
	    breachedCount: number;
	    breachCheckError: boolean;
	    weakItems: ItemRef[];
	    oldItems: ItemRef[];
	    missing2FAItems: ItemRef[];
	    breachedItems: ItemRef[];
	    duplicateGroups: DuplicateGroup[];
	
	    static createFrom(source: any = {}) {
	        return new HealthReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalItems = source["totalItems"];
	        this.totalPasswords = source["totalPasswords"];
	        this.totalScore = source["totalScore"];
	        this.weakCount = source["weakCount"];
	        this.duplicateCount = source["duplicateCount"];
	        this.oldCount = source["oldCount"];
	        this.missing2FA = source["missing2FA"];
	        this.breachedCount = source["breachedCount"];
	        this.breachCheckError = source["breachCheckError"];
	        this.weakItems = this.convertValues(source["weakItems"], ItemRef);
	        this.oldItems = this.convertValues(source["oldItems"], ItemRef);
	        this.missing2FAItems = this.convertValues(source["missing2FAItems"], ItemRef);
	        this.breachedItems = this.convertValues(source["breachedItems"], ItemRef);
	        this.duplicateGroups = this.convertValues(source["duplicateGroups"], DuplicateGroup);
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
	export class Item {
	    id: string;
	    type: string;
	    title: string;
	    username: string;
	    password: string;
	    url: string;
	    notes: string;
	    category: string;
	    tags: string[];
	    fields: Record<string, string>;
	    totpSecret: string;
	    favorite: boolean;
	    createdAt: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.title = source["title"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.category = source["category"];
	        this.tags = source["tags"];
	        this.fields = source["fields"];
	        this.totpSecret = source["totpSecret"];
	        this.favorite = source["favorite"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ImportResult {
	    created: number;
	    skipped: number;
	    preview: Item[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.created = source["created"];
	        this.skipped = source["skipped"];
	        this.preview = this.convertValues(source["preview"], Item);
	        this.errors = source["errors"];
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
	
	
	export class PasswordOptions {
	    length: number;
	    useUpper: boolean;
	    useLower: boolean;
	    useDigits: boolean;
	    useSymbols: boolean;
	    excludeAmbiguous: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PasswordOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.length = source["length"];
	        this.useUpper = source["useUpper"];
	        this.useLower = source["useLower"];
	        this.useDigits = source["useDigits"];
	        this.useSymbols = source["useSymbols"];
	        this.excludeAmbiguous = source["excludeAmbiguous"];
	    }
	}
	export class TOTPCode {
	    code: string;
	    secondsRemaining: number;
	
	    static createFrom(source: any = {}) {
	        return new TOTPCode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.secondsRemaining = source["secondsRemaining"];
	    }
	}
	export class TOTPSetupInfo {
	    secret: string;
	    qr: string;
	    otpauthURL: string;
	
	    static createFrom(source: any = {}) {
	        return new TOTPSetupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.secret = source["secret"];
	        this.qr = source["qr"];
	        this.otpauthURL = source["otpauthURL"];
	    }
	}
	export class VaultInfo {
	    name: string;
	    file: string;
	    lastOpened: number;
	
	    static createFrom(source: any = {}) {
	        return new VaultInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.file = source["file"];
	        this.lastOpened = source["lastOpened"];
	    }
	}
	export class VersionEntry {
	    id: string;
	    timestamp: number;
	    item: Item;
	
	    static createFrom(source: any = {}) {
	        return new VersionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.item = this.convertValues(source["item"], Item);
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

