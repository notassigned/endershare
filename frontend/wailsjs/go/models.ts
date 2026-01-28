export namespace main {
	
	export class FolderItem {
	    type: string;
	    name: string;
	    folderId: number;
	    size: number;
	    modifiedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new FolderItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.folderId = source["folderId"];
	        this.size = source["size"];
	        this.modifiedAt = source["modifiedAt"];
	    }
	}
	export class PathSegment {
	    name: string;
	    folderId: number;
	
	    static createFrom(source: any = {}) {
	        return new PathSegment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.folderId = source["folderId"];
	    }
	}
	export class PeerInfo {
	    peerId: string;
	    isOnline: boolean;
	    lastSeen: string;
	
	    static createFrom(source: any = {}) {
	        return new PeerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peerId = source["peerId"];
	        this.isOnline = source["isOnline"];
	        this.lastSeen = source["lastSeen"];
	    }
	}
	export class StorageStats {
	    entryCount: number;
	    totalSize: number;
	
	    static createFrom(source: any = {}) {
	        return new StorageStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entryCount = source["entryCount"];
	        this.totalSize = source["totalSize"];
	    }
	}

}

