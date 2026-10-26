export namespace api {
	
	export class CorpHistoryEntry {
	    CorporationID: number;
	    Name: string;
	    IconUrl: string;
	    Start: string;
	    End: string;
	
	    static createFrom(source: any = {}) {
	        return new CorpHistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CorporationID = source["CorporationID"];
	        this.Name = source["Name"];
	        this.IconUrl = source["IconUrl"];
	        this.Start = source["Start"];
	        this.End = source["End"];
	    }
	}
	export class DetailRow {
	    Label: string;
	    Value: string;
	    Security?: number;
	
	    static createFrom(source: any = {}) {
	        return new DetailRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Label = source["Label"];
	        this.Value = source["Value"];
	        this.Security = source["Security"];
	    }
	}
	export class HistoryEntry {
	    ID: number;
	    Name: string;
	    IconUrl: string;
	    Start: string;
	    End: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.IconUrl = source["IconUrl"];
	        this.Start = source["Start"];
	        this.End = source["End"];
	    }
	}
	export class MemberCorp {
	    CorporationID: number;
	    Name: string;
	    IconUrl: string;
	    MemberCount: number;
	
	    static createFrom(source: any = {}) {
	        return new MemberCorp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CorporationID = source["CorporationID"];
	        this.Name = source["Name"];
	        this.IconUrl = source["IconUrl"];
	        this.MemberCount = source["MemberCount"];
	    }
	}
	export class RelatedEntity {
	    Title: string;
	    Name: string;
	    ID: number;
	    Type: number;
	    TypeName: string;
	    ImageUrl: string;
	    ZkillUrl: string;
	    EveWhoUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new RelatedEntity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Title = source["Title"];
	        this.Name = source["Name"];
	        this.ID = source["ID"];
	        this.Type = source["Type"];
	        this.TypeName = source["TypeName"];
	        this.ImageUrl = source["ImageUrl"];
	        this.ZkillUrl = source["ZkillUrl"];
	        this.EveWhoUrl = source["EveWhoUrl"];
	    }
	}
	export class SystemLink {
	    SystemID: number;
	    Name: string;
	    SecurityStatus: number;
	
	    static createFrom(source: any = {}) {
	        return new SystemLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SystemID = source["SystemID"];
	        this.Name = source["Name"];
	        this.SecurityStatus = source["SecurityStatus"];
	    }
	}
	export class SystemStatic {
	    code: string;
	    destination: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemStatic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.destination = source["destination"];
	    }
	}
	export class TargetDisplay {
	    Name: string;
	    Type: number;
	    TargetImageUrl: string;
	    ZkillUrl: string;
	    EveWhoUrl: string;
	    DotlanUrl: string;
	    Related: RelatedEntity[];
	    CorpHistory: CorpHistoryEntry[];
	    AllianceHistory: HistoryEntry[];
	    Statics: SystemStatic[];
	    Stargates: SystemLink[];
	    MemberCorps: MemberCorp[];
	    DetailRows: DetailRow[];
	    SpaceLabel: string;
	    LeftDetailsTitle: string;
	    LeftDetailsValue: string;
	    RightDetailsTitle: string;
	    RightDetailsValue: string;
	
	    static createFrom(source: any = {}) {
	        return new TargetDisplay(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Type = source["Type"];
	        this.TargetImageUrl = source["TargetImageUrl"];
	        this.ZkillUrl = source["ZkillUrl"];
	        this.EveWhoUrl = source["EveWhoUrl"];
	        this.DotlanUrl = source["DotlanUrl"];
	        this.Related = this.convertValues(source["Related"], RelatedEntity);
	        this.CorpHistory = this.convertValues(source["CorpHistory"], CorpHistoryEntry);
	        this.AllianceHistory = this.convertValues(source["AllianceHistory"], HistoryEntry);
	        this.Statics = this.convertValues(source["Statics"], SystemStatic);
	        this.Stargates = this.convertValues(source["Stargates"], SystemLink);
	        this.MemberCorps = this.convertValues(source["MemberCorps"], MemberCorp);
	        this.DetailRows = this.convertValues(source["DetailRows"], DetailRow);
	        this.SpaceLabel = source["SpaceLabel"];
	        this.LeftDetailsTitle = source["LeftDetailsTitle"];
	        this.LeftDetailsValue = source["LeftDetailsValue"];
	        this.RightDetailsTitle = source["RightDetailsTitle"];
	        this.RightDetailsValue = source["RightDetailsValue"];
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

export namespace gui {
	
	export class CacheInfo {
	    path: string;
	    isDefault: boolean;
	    entries: number;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new CacheInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.isDefault = source["isDefault"];
	        this.entries = source["entries"];
	        this.sizeBytes = source["sizeBytes"];
	    }
	}
	export class CapitalPilot {
	    id: number;
	    name: string;
	    corporation: string;
	    alliance: string;
	    corporationIcon: string;
	    allianceIcon: string;
	    left: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CapitalPilot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.corporation = source["corporation"];
	        this.alliance = source["alliance"];
	        this.corporationIcon = source["corporationIcon"];
	        this.allianceIcon = source["allianceIcon"];
	        this.left = source["left"];
	    }
	}
	export class CapitalShip {
	    id: number;
	    name: string;
	    imageUrl: string;
	    pilots: CapitalPilot[];
	
	    static createFrom(source: any = {}) {
	        return new CapitalShip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.imageUrl = source["imageUrl"];
	        this.pilots = this.convertValues(source["pilots"], CapitalPilot);
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
	export class CorporationSummary {
	    id: number;
	    name: string;
	    imageUrl: string;
	    alliance: string;
	    allianceIcon: string;
	    zkillUrl: string;
	    evewhoUrl: string;
	    killmails: number;
	    kills: number;
	    losses: number;
	    pilots: number;
	    uniqueDays: number;
	    // Go type: time
	    firstSeen: any;
	    // Go type: time
	    lastSeen: any;
	
	    static createFrom(source: any = {}) {
	        return new CorporationSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.imageUrl = source["imageUrl"];
	        this.alliance = source["alliance"];
	        this.allianceIcon = source["allianceIcon"];
	        this.zkillUrl = source["zkillUrl"];
	        this.evewhoUrl = source["evewhoUrl"];
	        this.killmails = source["killmails"];
	        this.kills = source["kills"];
	        this.losses = source["losses"];
	        this.pilots = source["pilots"];
	        this.uniqueDays = source["uniqueDays"];
	        this.firstSeen = this.convertValues(source["firstSeen"], null);
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
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
	export class Fleetmate {
	    id: number;
	    name: string;
	    imageUrl: string;
	    killmailsShared: number;
	    uniqueDays: number;
	    corporation: string;
	    alliance: string;
	    corporationIcon: string;
	    allianceIcon: string;
	    zkillUrl: string;
	    evewhoUrl: string;
	    // Go type: time
	    firstTogether: any;
	    // Go type: time
	    lastTogether: any;
	
	    static createFrom(source: any = {}) {
	        return new Fleetmate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.imageUrl = source["imageUrl"];
	        this.killmailsShared = source["killmailsShared"];
	        this.uniqueDays = source["uniqueDays"];
	        this.corporation = source["corporation"];
	        this.alliance = source["alliance"];
	        this.corporationIcon = source["corporationIcon"];
	        this.allianceIcon = source["allianceIcon"];
	        this.zkillUrl = source["zkillUrl"];
	        this.evewhoUrl = source["evewhoUrl"];
	        this.firstTogether = this.convertValues(source["firstTogether"], null);
	        this.lastTogether = this.convertValues(source["lastTogether"], null);
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
	export class SystemSummary {
	    id: number;
	    name: string;
	    securityStatus: number;
	    region: string;
	    class: string;
	    statics: api.SystemStatic[];
	    zkillUrl: string;
	    dotlanUrl: string;
	    killmails: number;
	    kills: number;
	    losses: number;
	    uniqueDays: number;
	    // Go type: time
	    firstSeen: any;
	    // Go type: time
	    lastSeen: any;
	
	    static createFrom(source: any = {}) {
	        return new SystemSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.securityStatus = source["securityStatus"];
	        this.region = source["region"];
	        this.class = source["class"];
	        this.statics = this.convertValues(source["statics"], api.SystemStatic);
	        this.zkillUrl = source["zkillUrl"];
	        this.dotlanUrl = source["dotlanUrl"];
	        this.killmails = source["killmails"];
	        this.kills = source["kills"];
	        this.losses = source["losses"];
	        this.uniqueDays = source["uniqueDays"];
	        this.firstSeen = this.convertValues(source["firstSeen"], null);
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
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
	export class KillmailCard {
	    id: number;
	    url: string;
	    // Go type: time
	    time: any;
	    outcome: string;
	    solo: boolean;
	    totalValue: number;
	    droppedValue: number;
	    labels: string[];
	    systemName: string;
	    securityStatus: number;
	    region: string;
	    class: string;
	    shipName: string;
	    shipImageUrl: string;
	    victimName: string;
	    victimCorp: string;
	    victimAlliance: string;
	    finalBlowName: string;
	    finalBlowCorp: string;
	    finalBlowAlliance: string;
	    victimCorpIcon: string;
	    victimAllianceIcon: string;
	    finalBlowCorpIcon: string;
	    finalBlowAllianceIcon: string;
	    attackerCount: number;
	
	    static createFrom(source: any = {}) {
	        return new KillmailCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.time = this.convertValues(source["time"], null);
	        this.outcome = source["outcome"];
	        this.solo = source["solo"];
	        this.totalValue = source["totalValue"];
	        this.droppedValue = source["droppedValue"];
	        this.labels = source["labels"];
	        this.systemName = source["systemName"];
	        this.securityStatus = source["securityStatus"];
	        this.region = source["region"];
	        this.class = source["class"];
	        this.shipName = source["shipName"];
	        this.shipImageUrl = source["shipImageUrl"];
	        this.victimName = source["victimName"];
	        this.victimCorp = source["victimCorp"];
	        this.victimAlliance = source["victimAlliance"];
	        this.finalBlowName = source["finalBlowName"];
	        this.finalBlowCorp = source["finalBlowCorp"];
	        this.finalBlowAlliance = source["finalBlowAlliance"];
	        this.victimCorpIcon = source["victimCorpIcon"];
	        this.victimAllianceIcon = source["victimAllianceIcon"];
	        this.finalBlowCorpIcon = source["finalBlowCorpIcon"];
	        this.finalBlowAllianceIcon = source["finalBlowAllianceIcon"];
	        this.attackerCount = source["attackerCount"];
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
	export class IntelReport {
	    killmails: KillmailCard[];
	    fleetmates: Fleetmate[];
	    members: Fleetmate[];
	    pilots: Fleetmate[];
	    corporations: CorporationSummary[];
	    systems: SystemSummary[];
	    capitals: CapitalShip[];
	    memberList: boolean;
	    sources: string;
	
	    static createFrom(source: any = {}) {
	        return new IntelReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.killmails = this.convertValues(source["killmails"], KillmailCard);
	        this.fleetmates = this.convertValues(source["fleetmates"], Fleetmate);
	        this.members = this.convertValues(source["members"], Fleetmate);
	        this.pilots = this.convertValues(source["pilots"], Fleetmate);
	        this.corporations = this.convertValues(source["corporations"], CorporationSummary);
	        this.systems = this.convertValues(source["systems"], SystemSummary);
	        this.capitals = this.convertValues(source["capitals"], CapitalShip);
	        this.memberList = source["memberList"];
	        this.sources = source["sources"];
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
	
	export class MemberCorpRow {
	    id: number;
	    name: string;
	    iconUrl: string;
	    memberCount: number;
	    timeInAlliance: string;
	    zkillUrl: string;
	    evewhoUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new MemberCorpRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.iconUrl = source["iconUrl"];
	        this.memberCount = source["memberCount"];
	        this.timeInAlliance = source["timeInAlliance"];
	        this.zkillUrl = source["zkillUrl"];
	        this.evewhoUrl = source["evewhoUrl"];
	    }
	}
	export class MemberRow {
	    id: number;
	    name: string;
	    imageUrl: string;
	    // Go type: time
	    lastSeen?: any;
	    corporation: string;
	    corporationIcon: string;
	    born: string;
	    timeInCorp: string;
	    npcTime: string;
	    left: boolean;
	    zkillUrl: string;
	    evewhoUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new MemberRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.imageUrl = source["imageUrl"];
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	        this.corporation = source["corporation"];
	        this.corporationIcon = source["corporationIcon"];
	        this.born = source["born"];
	        this.timeInCorp = source["timeInCorp"];
	        this.npcTime = source["npcTime"];
	        this.left = source["left"];
	        this.zkillUrl = source["zkillUrl"];
	        this.evewhoUrl = source["evewhoUrl"];
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
	export class MemberPage {
	    rows: MemberRow[];
	    corporations: MemberCorpRow[];
	    total: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MemberPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], MemberRow);
	        this.corporations = this.convertValues(source["corporations"], MemberCorpRow);
	        this.total = source["total"];
	        this.hasMore = source["hasMore"];
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
	
	export class Preferences {
	    includeZkillLabels: boolean;
	    maxKillmails: number;
	    killmailsNewerThanDays: number;
	    useCachedKillmails: boolean;
	    timeZone: string;
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includeZkillLabels = source["includeZkillLabels"];
	        this.maxKillmails = source["maxKillmails"];
	        this.killmailsNewerThanDays = source["killmailsNewerThanDays"];
	        this.useCachedKillmails = source["useCachedKillmails"];
	        this.timeZone = source["timeZone"];
	    }
	}
	export class Settings {
	    theme: string;
	    cacheDir?: string;
	    includeZkillLabels: boolean;
	    maxKillmails: number;
	    killmailsNewerThanDays: number;
	    useCachedKillmails: boolean;
	    timeZone: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.cacheDir = source["cacheDir"];
	        this.includeZkillLabels = source["includeZkillLabels"];
	        this.maxKillmails = source["maxKillmails"];
	        this.killmailsNewerThanDays = source["killmailsNewerThanDays"];
	        this.useCachedKillmails = source["useCachedKillmails"];
	        this.timeZone = source["timeZone"];
	    }
	}
	
	export class TargetResult {
	    id: number;
	    type: string;
	    display: api.TargetDisplay;
	
	    static createFrom(source: any = {}) {
	        return new TargetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.display = this.convertValues(source["display"], api.TargetDisplay);
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

