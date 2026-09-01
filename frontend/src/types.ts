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
}

export interface ShelfStatus {
  paths: { inbox: string; inboxes: string[]; library: string; appData: string; trash: string };
  desktop: { available: boolean; autoStart: boolean };
  scanner: { running: boolean; pending: number; lastError?: string; lastErrorFile?: string };
}
