export namespace app {
	
	export class State {
	    Ctx: any;
	    Archive: any;
	    Runtime: any;
	    Filesystem: any;
	    Batch: any;
	    Converter: any;
	    OfficeConverter: any;
	    DirStream: any;
	    Utility: any;
	    Navigation: any;
	    Workspace: any;
	    Operations: any;
	    DirWatch: any;
	    Elevation: any;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Ctx = source["Ctx"];
	        this.Archive = source["Archive"];
	        this.Runtime = source["Runtime"];
	        this.Filesystem = source["Filesystem"];
	        this.Batch = source["Batch"];
	        this.Converter = source["Converter"];
	        this.OfficeConverter = source["OfficeConverter"];
	        this.DirStream = source["DirStream"];
	        this.Utility = source["Utility"];
	        this.Navigation = source["Navigation"];
	        this.Workspace = source["Workspace"];
	        this.Operations = source["Operations"];
	        this.DirWatch = source["DirWatch"];
	        this.Elevation = source["Elevation"];
	    }
	}
	export class VideoPlaybackPlan {
	    mode: string;
	    format: string;
	    videoCodec?: string;
	    audioCodec?: string;
	    durationSeconds: number;
	    zeroDelay: boolean;
	    seekable: boolean;
	    useNativePlayer: boolean;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoPlaybackPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.format = source["format"];
	        this.videoCodec = source["videoCodec"];
	        this.audioCodec = source["audioCodec"];
	        this.durationSeconds = source["durationSeconds"];
	        this.zeroDelay = source["zeroDelay"];
	        this.seekable = source["seekable"];
	        this.useNativePlayer = source["useNativePlayer"];
	        this.url = source["url"];
	    }
	}

}

export namespace cache {
	
	export class Cache {
	
	
	    static createFrom(source: any = {}) {
	        return new Cache(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace contracts {
	
	export class ArchiveEntriesRequest {
	    fsPath: string;
	    nestedChain: string[];
	    entries: string[];
	    stripPrefix?: string;
	    destinationDir: string;
	    flattenPaths: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ArchiveEntriesRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fsPath = source["fsPath"];
	        this.nestedChain = source["nestedChain"];
	        this.entries = source["entries"];
	        this.stripPrefix = source["stripPrefix"];
	        this.destinationDir = source["destinationDir"];
	        this.flattenPaths = source["flattenPaths"];
	    }
	}
	export class BatchItemError {
	    source: string;
	    destination?: string;
	    error: string;
	    code: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchItemError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.destination = source["destination"];
	        this.error = source["error"];
	        this.code = source["code"];
	    }
	}
	export class ArchiveEntriesResult {
	    operationId: string;
	    total: number;
	    succeeded: number;
	    failed: number;
	    sourceRemoved: boolean;
	    errors?: BatchItemError[];
	
	    static createFrom(source: any = {}) {
	        return new ArchiveEntriesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.total = source["total"];
	        this.succeeded = source["succeeded"];
	        this.failed = source["failed"];
	        this.sourceRemoved = source["sourceRemoved"];
	        this.errors = this.convertValues(source["errors"], BatchItemError);
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
	export class FileInfo {
	    name: string;
	    path: string;
	    type: string;
	    mimeType?: string;
	    size: number;
	    created: number;
	    modified: number;
	    accessed: number;
	    permissions: string;
	    isHidden: boolean;
	    isSymlink?: boolean;
	    isNested: boolean;
	    nestedChain?: string[];
	    registryKind?: string;
	    registryValueType?: string;
	    registryPreview?: string;
	    searchBasePath?: string;
	    searchPattern?: string;
	    searchParentPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.mimeType = source["mimeType"];
	        this.size = source["size"];
	        this.created = source["created"];
	        this.modified = source["modified"];
	        this.accessed = source["accessed"];
	        this.permissions = source["permissions"];
	        this.isHidden = source["isHidden"];
	        this.isSymlink = source["isSymlink"];
	        this.isNested = source["isNested"];
	        this.nestedChain = source["nestedChain"];
	        this.registryKind = source["registryKind"];
	        this.registryValueType = source["registryValueType"];
	        this.registryPreview = source["registryPreview"];
	        this.searchBasePath = source["searchBasePath"];
	        this.searchPattern = source["searchPattern"];
	        this.searchParentPath = source["searchParentPath"];
	    }
	}
	export class ArchiveListing {
	    archivePath: string;
	    innerPath: string;
	    nestedChain?: string[];
	    entries: FileInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ArchiveListing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.archivePath = source["archivePath"];
	        this.innerPath = source["innerPath"];
	        this.nestedChain = source["nestedChain"];
	        this.entries = this.convertValues(source["entries"], FileInfo);
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
	export class BatchDeleteRequest {
	    paths: string[];
	    recursive: boolean;
	    useTrash?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchDeleteRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paths = source["paths"];
	        this.recursive = source["recursive"];
	        this.useTrash = source["useTrash"];
	    }
	}
	export class BatchExtractRequest {
	    archives: string[];
	    destinationDir: string;
	    createSubdirectory: boolean;
	    expandNestedArchives: boolean;
	    maxNestDepth: number;
	    flattenSingleRoot: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchExtractRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.archives = source["archives"];
	        this.destinationDir = source["destinationDir"];
	        this.createSubdirectory = source["createSubdirectory"];
	        this.expandNestedArchives = source["expandNestedArchives"];
	        this.maxNestDepth = source["maxNestDepth"];
	        this.flattenSingleRoot = source["flattenSingleRoot"];
	    }
	}
	export class BatchExtractResult {
	    operationId: string;
	    total: number;
	    succeeded: number;
	    skipped: number;
	    failed: number;
	    extractedFiles: number;
	    extractedBytes: number;
	    errors?: BatchItemError[];
	
	    static createFrom(source: any = {}) {
	        return new BatchExtractResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.total = source["total"];
	        this.succeeded = source["succeeded"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.extractedFiles = source["extractedFiles"];
	        this.extractedBytes = source["extractedBytes"];
	        this.errors = this.convertValues(source["errors"], BatchItemError);
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
	export class BatchItem {
	    source: string;
	    destination: string;
	    exactDestination?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.destination = source["destination"];
	        this.exactDestination = source["exactDestination"];
	    }
	}
	
	export class BatchRequest {
	    items: BatchItem[];
	    recursive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], BatchItem);
	        this.recursive = source["recursive"];
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
	export class BatchResult {
	    operationId: string;
	    total: number;
	    succeeded: number;
	    skipped: number;
	    failed: number;
	    errors?: BatchItemError[];
	
	    static createFrom(source: any = {}) {
	        return new BatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.total = source["total"];
	        this.succeeded = source["succeeded"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.errors = this.convertValues(source["errors"], BatchItemError);
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
	export class SelectiveExtractItem {
	    archivePath: string;
	    nestedChain: string[];
	    entries: string[];
	    stripPrefix?: string;
	    destinationDir?: string;
	    flattenPaths: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SelectiveExtractItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.archivePath = source["archivePath"];
	        this.nestedChain = source["nestedChain"];
	        this.entries = source["entries"];
	        this.stripPrefix = source["stripPrefix"];
	        this.destinationDir = source["destinationDir"];
	        this.flattenPaths = source["flattenPaths"];
	    }
	}
	export class BatchSelectiveExtractRequest {
	    items: SelectiveExtractItem[];
	    destinationDir: string;
	
	    static createFrom(source: any = {}) {
	        return new BatchSelectiveExtractRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], SelectiveExtractItem);
	        this.destinationDir = source["destinationDir"];
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
	export class BatchSelectiveExtractResult {
	    operationId: string;
	    total: number;
	    succeeded: number;
	    skipped: number;
	    failed: number;
	    extractedFiles: number;
	    extractedBytes: number;
	    errors?: BatchItemError[];
	
	    static createFrom(source: any = {}) {
	        return new BatchSelectiveExtractResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.total = source["total"];
	        this.succeeded = source["succeeded"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.extractedFiles = source["extractedFiles"];
	        this.extractedBytes = source["extractedBytes"];
	        this.errors = this.convertValues(source["errors"], BatchItemError);
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
	export class BrowserPaneSyncRequest {
	    surfaceId: string;
	    url: string;
	    theme: string;
	    backgroundColor: string;
	    visible: boolean;
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new BrowserPaneSyncRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.surfaceId = source["surfaceId"];
	        this.url = source["url"];
	        this.theme = source["theme"];
	        this.backgroundColor = source["backgroundColor"];
	        this.visible = source["visible"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class ConflictEvent {
	    operationId: string;
	    conflictId: string;
	    sourcePath: string;
	    destPath: string;
	    sourceSize: number;
	    destSize: number;
	    sourceMtime: number;
	    destMtime: number;
	    isDirectory: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConflictEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.conflictId = source["conflictId"];
	        this.sourcePath = source["sourcePath"];
	        this.destPath = source["destPath"];
	        this.sourceSize = source["sourceSize"];
	        this.destSize = source["destSize"];
	        this.sourceMtime = source["sourceMtime"];
	        this.destMtime = source["destMtime"];
	        this.isDirectory = source["isDirectory"];
	    }
	}
	export class DirectoryListing {
	    path: string;
	    entries: FileInfo[];
	    totalSize: number;
	
	    static createFrom(source: any = {}) {
	        return new DirectoryListing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.entries = this.convertValues(source["entries"], FileInfo);
	        this.totalSize = source["totalSize"];
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
	export class DiskUsageTypeStat {
	    count: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsageTypeStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.size = source["size"];
	    }
	}
	export class DiskUsageEntry {
	    name: string;
	    path: string;
	    size: number;
	    isDir: boolean;
	    files: number;
	    dirs: number;
	    types?: Record<string, DiskUsageTypeStat>;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsageEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.isDir = source["isDir"];
	        this.files = source["files"];
	        this.dirs = source["dirs"];
	        this.types = this.convertValues(source["types"], DiskUsageTypeStat, true);
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
	export class DiskUsageResult {
	    path: string;
	    totalSize: number;
	    totalFiles: number;
	    totalDirs: number;
	    types?: Record<string, DiskUsageTypeStat>;
	    entries: DiskUsageEntry[];
	    elapsedMs: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.totalSize = source["totalSize"];
	        this.totalFiles = source["totalFiles"];
	        this.totalDirs = source["totalDirs"];
	        this.types = this.convertValues(source["types"], DiskUsageTypeStat, true);
	        this.entries = this.convertValues(source["entries"], DiskUsageEntry);
	        this.elapsedMs = source["elapsedMs"];
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
	
	export class DriveInfo {
	    name: string;
	    path: string;
	    schemeType: string;
	    totalSpace: number;
	    freeSpace: number;
	    fileSystem?: string;
	
	    static createFrom(source: any = {}) {
	        return new DriveInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.schemeType = source["schemeType"];
	        this.totalSpace = source["totalSpace"];
	        this.freeSpace = source["freeSpace"];
	        this.fileSystem = source["fileSystem"];
	    }
	}
	export class DrivesList {
	    schemes: DriveInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DrivesList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemes = this.convertValues(source["schemes"], DriveInfo);
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
	export class FileContent {
	    path: string;
	    content: string;
	    encoding: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.encoding = source["encoding"];
	        this.size = source["size"];
	    }
	}
	
	export class FileProbe {
	    mimeType: string;
	    encoding?: string;
	    isText: boolean;
	    isArchive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileProbe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mimeType = source["mimeType"];
	        this.encoding = source["encoding"];
	        this.isText = source["isText"];
	        this.isArchive = source["isArchive"];
	    }
	}
	export class FilterCriteria {
	    type: string;
	    value: string;
	    operator?: string;
	
	    static createFrom(source: any = {}) {
	        return new FilterCriteria(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.value = source["value"];
	        this.operator = source["operator"];
	    }
	}
	export class FilterQuery {
	    text?: string;
	    criteria?: FilterCriteria[];
	    caseSensitive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FilterQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.criteria = this.convertValues(source["criteria"], FilterCriteria);
	        this.caseSensitive = source["caseSensitive"];
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
	export class OSInfoResponse {
	    os: string;
	    version: string;
	    arch: string;
	    hostname: string;
	    systemDrive?: string;
	
	    static createFrom(source: any = {}) {
	        return new OSInfoResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.version = source["version"];
	        this.arch = source["arch"];
	        this.hostname = source["hostname"];
	        this.systemDrive = source["systemDrive"];
	    }
	}
	export class OperationProgress {
	    id: string;
	    type: string;
	    status: string;
	    title: string;
	    sourcePath: string;
	    destinationPath?: string;
	    totalItems: number;
	    processedItems: number;
	    totalBytes: number;
	    processedBytes: number;
	    currentFile: string;
	    currentFileSize: number;
	    percentComplete: number;
	    speedBytesPerSec: number;
	    avgSpeedBytesPerSec: number;
	    etaSeconds: number;
	    startTime: number;
	    elapsedSeconds: number;
	    endTime?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new OperationProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.title = source["title"];
	        this.sourcePath = source["sourcePath"];
	        this.destinationPath = source["destinationPath"];
	        this.totalItems = source["totalItems"];
	        this.processedItems = source["processedItems"];
	        this.totalBytes = source["totalBytes"];
	        this.processedBytes = source["processedBytes"];
	        this.currentFile = source["currentFile"];
	        this.currentFileSize = source["currentFileSize"];
	        this.percentComplete = source["percentComplete"];
	        this.speedBytesPerSec = source["speedBytesPerSec"];
	        this.avgSpeedBytesPerSec = source["avgSpeedBytesPerSec"];
	        this.etaSeconds = source["etaSeconds"];
	        this.startTime = source["startTime"];
	        this.elapsedSeconds = source["elapsedSeconds"];
	        this.endTime = source["endTime"];
	        this.error = source["error"];
	    }
	}
	export class OperationResult {
	    success: boolean;
	    message?: string;
	    affectedPaths?: string[];
	
	    static createFrom(source: any = {}) {
	        return new OperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.affectedPaths = source["affectedPaths"];
	    }
	}
	export class ViewerTab {
	    targetPath: string;
	    viewerId?: string;
	    autoFollow?: boolean;
	    sourcePane?: string;
	    sourceTabId?: string;
	    sourceDir?: string;
	
	    static createFrom(source: any = {}) {
	        return new ViewerTab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targetPath = source["targetPath"];
	        this.viewerId = source["viewerId"];
	        this.autoFollow = source["autoFollow"];
	        this.sourcePane = source["sourcePane"];
	        this.sourceTabId = source["sourceTabId"];
	        this.sourceDir = source["sourceDir"];
	    }
	}
	export class TabInfo {
	    id: string;
	    title: string;
	    path: string;
	    kind?: string;
	    viewer?: ViewerTab;
	    isClosable?: boolean;
	    isActive: boolean;
	    isPinned: boolean;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new TabInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.viewer = this.convertValues(source["viewer"], ViewerTab);
	        this.isClosable = source["isClosable"];
	        this.isActive = source["isActive"];
	        this.isPinned = source["isPinned"];
	        this.createdAt = source["createdAt"];
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
	export class PaneState {
	    id: string;
	    tabs: TabInfo[];
	    activeTabId: string;
	
	    static createFrom(source: any = {}) {
	        return new PaneState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.tabs = this.convertValues(source["tabs"], TabInfo);
	        this.activeTabId = source["activeTabId"];
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
	export class RemoteConfig {
	    protocol: string;
	    name?: string;
	    host: string;
	    port?: number;
	    username?: string;
	    password?: string;
	    domain?: string;
	    share?: string;
	    endpoint?: string;
	    region?: string;
	    bucket?: string;
	    prefix?: string;
	    accessKeyId?: string;
	    secretAccessKey?: string;
	    sessionToken?: string;
	    anonymous?: boolean;
	    usePathStyle?: boolean;
	    insecure?: boolean;
	    accessToken?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.domain = source["domain"];
	        this.share = source["share"];
	        this.endpoint = source["endpoint"];
	        this.region = source["region"];
	        this.bucket = source["bucket"];
	        this.prefix = source["prefix"];
	        this.accessKeyId = source["accessKeyId"];
	        this.secretAccessKey = source["secretAccessKey"];
	        this.sessionToken = source["sessionToken"];
	        this.anonymous = source["anonymous"];
	        this.usePathStyle = source["usePathStyle"];
	        this.insecure = source["insecure"];
	        this.accessToken = source["accessToken"];
	    }
	}
	export class RemoteOAuthBeginRequest {
	    protocol: string;
	    clientId: string;
	    tenant?: string;
	    scopes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new RemoteOAuthBeginRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.clientId = source["clientId"];
	        this.tenant = source["tenant"];
	        this.scopes = source["scopes"];
	    }
	}
	export class RemoteOAuthBeginResponse {
	    sessionId: string;
	    protocol: string;
	    authUrl: string;
	    redirectUri: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteOAuthBeginResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.protocol = source["protocol"];
	        this.authUrl = source["authUrl"];
	        this.redirectUri = source["redirectUri"];
	    }
	}
	export class RemoteOAuthStatus {
	    sessionId: string;
	    protocol: string;
	    status: string;
	    error?: string;
	    accountName?: string;
	    accountEmail?: string;
	    expiresAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new RemoteOAuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.protocol = source["protocol"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.accountName = source["accountName"];
	        this.accountEmail = source["accountEmail"];
	        this.expiresAt = source["expiresAt"];
	    }
	}
	export class RemoteOAuthConnectRequest {
	    sessionId: string;
	    name?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteOAuthConnectRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.name = source["name"];
	    }
	}
	export class RemoteMount {
	    protocol: string;
	    host: string;
	    name: string;
	    publicPath: string;
	    mountPoint: string;
	    readOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RemoteMount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.name = source["name"];
	        this.publicPath = source["publicPath"];
	        this.mountPoint = source["mountPoint"];
	        this.readOnly = source["readOnly"];
	    }
	}
	export class SearchResult {
	    path: string;
	    matches: FileInfo[];
	    totalMatches: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.matches = this.convertValues(source["matches"], FileInfo);
	        this.totalMatches = source["totalMatches"];
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
	
	export class SpecialFolder {
	    name: string;
	    path: string;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new SpecialFolder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.icon = source["icon"];
	    }
	}
	export class SpecialFoldersResponse {
	    folders: SpecialFolder[];
	
	    static createFrom(source: any = {}) {
	        return new SpecialFoldersResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folders = this.convertValues(source["folders"], SpecialFolder);
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
	
	export class UserPreferences {
	    theme: string;
	    defaultShowHidden: boolean;
	    defaultSortBy: string;
	    defaultSortOrder: string;
	    enableKeyboardShortcuts: boolean;
	    doubleClickAction: string;
	    confirmDelete: boolean;
	    memoryRoot?: string;
	
	    static createFrom(source: any = {}) {
	        return new UserPreferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.defaultShowHidden = source["defaultShowHidden"];
	        this.defaultSortBy = source["defaultSortBy"];
	        this.defaultSortOrder = source["defaultSortOrder"];
	        this.enableKeyboardShortcuts = source["enableKeyboardShortcuts"];
	        this.doubleClickAction = source["doubleClickAction"];
	        this.confirmDelete = source["confirmDelete"];
	        this.memoryRoot = source["memoryRoot"];
	    }
	}
	
	export class WorkspaceConfig {
	    id: string;
	    name: string;
	    icon?: string;
	    color?: string;
	    leftPane: PaneState;
	    rightPane: PaneState;
	    showHidden: boolean;
	    createdAt: number;
	    lastAccessedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.icon = source["icon"];
	        this.color = source["color"];
	        this.leftPane = this.convertValues(source["leftPane"], PaneState);
	        this.rightPane = this.convertValues(source["rightPane"], PaneState);
	        this.showHidden = source["showHidden"];
	        this.createdAt = source["createdAt"];
	        this.lastAccessedAt = source["lastAccessedAt"];
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
	export class WorkspaceList {
	    workspaces: WorkspaceConfig[];
	    activeWorkspaceId: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaces = this.convertValues(source["workspaces"], WorkspaceConfig);
	        this.activeWorkspaceId = source["activeWorkspaceId"];
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

export namespace elevation {
	
	export class SessionStatus {
	    active: boolean;
	    pid?: number;
	    startedAt?: number;
	    port?: number;
	    opCount: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.pid = source["pid"];
	        this.startedAt = source["startedAt"];
	        this.port = source["port"];
	        this.opCount = source["opCount"];
	        this.error = source["error"];
	    }
	}

}

export namespace ops {
	
	export class ActiveOperation {
	    ID: string;
	    Type: string;
	    Progress: contracts.OperationProgress;
	
	    static createFrom(source: any = {}) {
	        return new ActiveOperation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Type = source["Type"];
	        this.Progress = this.convertValues(source["Progress"], contracts.OperationProgress);
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

export namespace utility {
	
	export class ResolvedLink {
	    target: string;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ResolvedLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.isDir = source["isDir"];
	    }
	}

}
