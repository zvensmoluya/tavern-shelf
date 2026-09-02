export interface CharacterProfile {
  name: string;
  nickname?: string;
  creator?: string;
  characterVersion?: string;
  tags: string[];
  description?: string;
  personality?: string;
  scenario?: string;
  messageExample?: string;
  creatorNotes?: string;
  creatorNotesMultilingual?: Record<string, string>;
  systemPrompt?: string;
  postHistoryInstructions?: string;
}

export interface Greetings {
  firstMessage?: string;
  alternate: string[];
  groupOnly: string[];
  totalCount: number;
  alternateCount: number;
  groupOnlyCount: number;
}

export interface CharacterBookEntry {
  name: string;
  comment?: string;
  keys: string[];
  secondaryKeys: string[];
  content?: string;
  enabled: boolean;
  constant: boolean;
  selective: boolean;
  useRegex: boolean;
  caseSensitive: boolean;
  insertionOrder: number;
}

export interface CharacterBook {
  name?: string;
  description?: string;
  entryCount: number;
  enabledEntryCount: number;
  scanDepth?: number;
  tokenBudget?: number;
  recursiveScanning?: boolean;
  entries: CharacterBookEntry[];
}

export interface RegexScript {
  name: string;
  findRegex?: string;
  replaceString?: string;
  placement: number[];
  disabled: boolean;
  markdownOnly: boolean;
  promptOnly: boolean;
  runOnEdit: boolean;
  minDepth?: number;
  maxDepth?: number;
}

export interface Manifest {
  schemaVersion: number;
  character: CharacterProfile;
  greetings: Greetings;
  characterBook?: CharacterBook;
  regexScripts: RegexScript[];
  extensions: Array<{ name: string; kind: string }>;
  assets: Array<{ type: string; name: string; ext?: string; uriKind?: string }>;
  sources: string[];
  interaction: {
    hasHtml: boolean;
    hasJavaScript: boolean;
    hasInteractiveExtension: boolean;
    markers: string[];
  };
  creationDate?: number;
  modifiedDate?: number;
}

export interface Character {
  id: string;
  sourceHash?: string;
  name: string;
  creator?: string;
  spec?: string;
  specVersion?: string;
  tags: string[];
  hasWorldbook: boolean;
  hasRegex: boolean;
  hasExtensions: boolean;
  hasInteractive: boolean;
  sourceFormat: string;
  sourceIsImage: boolean;
  sourceFilename: string;
  sourceSize: number;
  importedAt: string;
  avatarUrl?: string;
  sourceUrl: string;
  manifest: Manifest;
  favorite: boolean;
  note?: string;
  collectionIds: string[];
}

export interface Collection {
  id: string;
  name: string;
  characterCount: number;
  createdAt: string;
}

export interface CharacterOrganization {
  favorite: boolean;
  note: string;
  collectionIds: string[];
}

export interface PresetField {
  key: string;
  label: string;
  value: string;
}

export interface PresetManifest {
  type: string;
  fieldCount: number;
  promptCount?: number;
  fields: PresetField[];
  textBlocks: PresetField[];
}

export interface ShelfResource {
  id: string;
  sourceHash: string;
  kind: "worldbook" | "preset";
  subtype?: string;
  name: string;
  description?: string;
  sourceFilename: string;
  sourceSize: number;
  importedAt: string;
  sourceUrl: string;
  worldbook?: CharacterBook;
  preset?: PresetManifest;
}

export type LibrarySection = "characters" | "worldbooks" | "presets";

export interface TransferTarget {
  kind: "character" | "worldbook" | "preset";
  id: string;
  name: string;
}

export interface TransferSession {
  id: string;
  protocol: "tavern-shelf-transfer";
  version: number;
  kind: TransferTarget["kind"];
  subtype?: string;
  name: string;
  filename: string;
  size: number;
  sha256: string;
  url: string;
  addresses: string[];
  expiresAt: string;
}

export interface ShelfStatus {
  paths: {
    inbox: string;
    inboxes: string[];
    inboxDetails: Array<{ path: string; mode: "move" | "copy" }>;
    library: string;
    appData: string;
    trash: string;
  };
  desktop: { available: boolean; autoStart: boolean };
	scanner: {
		running: boolean;
		pending: number;
		lastError?: string;
		lastErrorFile?: string;
		failures: Array<{ path: string; file: string; error: string; lastAttemptAt: string; nextRetryAt: string }>;
	};
	oneShotScan: {
		id?: string;
		directory?: string;
		running: boolean;
		total: number;
		imported: number;
		duplicates: number;
		failed: number;
		startedAt?: string;
		completedAt?: string;
		issues?: Array<{ file: string; error: string }>;
	};
}

export interface TrashItem {
	id: string;
	kind: "character" | "worldbook" | "preset" | "unknown";
	name: string;
	sourceFilename: string;
	sourceSize: number;
	deletedAt: string;
	error?: string;
}

export interface RestoreSummary {
	total: number;
	imported: number;
	duplicates: number;
	failed: number;
	issues?: Array<{ file: string; error: string }>;
}

export interface ImportResult {
	id: string;
	kind: "character" | "worldbook" | "preset";
	name: string;
	duplicate: boolean;
}

export interface ConnectorStatus {
  protocol: "tavern-shelf-connector";
  version: number;
  listening: boolean;
  address?: string;
  listenerError?: string;
  paired: boolean;
  client?: { name: string; version?: string; pairedAt: string };
  pairingExpiresAt?: string;
}

export interface ConnectorPairing {
  code: string;
  expiresAt: string;
}
