export type ItemType = 'login' | 'note' | 'credit_card' | 'identity' | 'passkey'

export interface PasskeyData {
    credentialId: string
    rpId: string
    rpName: string
    userHandle: string
    username: string
    displayName: string
    privateKey: string
    publicKey: string
    coseAlg: number
    transports: string[]
    aaguid: string
    backupState: string
}

export interface Item {
    id: string
    type: ItemType
    title: string
    username: string
    password: string
    url: string
    notes: string
    category: string
    tags: string[]
    fields: Record<string, string>
    totpSecret: string
    passkey?: PasskeyData | null
    favorite: boolean
    deleted: boolean
    deletedAt: number
    createdAt: number
    updatedAt: number
}

export interface PasswordOptions {
    length: number
    useUpper: boolean
    useLower: boolean
    useDigits: boolean
    useSymbols: boolean
    excludeAmbiguous: boolean
}

export interface Attachment {
    id: string
    name: string
    size: number
    addedAt: number
}

export interface AttachmentPayload {
    name: string
    data: string
}

export interface SyncStatus {
    configured: boolean
    provider: string
    remote: string
    state: string
    lastSync: number
    detail: string
    conflict: string
}

export interface VersionEntry {
    id: string
    timestamp: number
    item: Item
}

export interface FieldMapping {
    column: number
    field: string
}

export interface ImportResult {
    created: number
    skipped: number
    preview: Item[]
    errors: string[]
}

export interface VaultSettings {
    autolockMinutes: string
    defaultType: string
}

export interface VaultInfo {
    name: string
    file: string
    lastOpened: number
}

export interface TOTPSetupInfo {
    secret: string
    qr: string
    otpauthURL: string
}

export interface TOTPCode {
    code: string
    secondsRemaining: number
}

export interface ItemRef {
    id: string
    title: string
    score: number
}

export interface DuplicateGroup {
    password: string
    items: ItemRef[]
}

export interface HealthReport {
    totalItems: number
    totalPasswords: number
    totalScore: number
    weakCount: number
    duplicateCount: number
    oldCount: number
    missing2FA: number
    breachedCount: number
    breachCheckError: boolean
    weakItems: ItemRef[]
    oldItems: ItemRef[]
    missing2FAItems: ItemRef[]
    breachedItems: ItemRef[]
    duplicateGroups: DuplicateGroup[]
}

export type Phase = 'loading' | 'setup' | 'unlock' | 'main'

export interface AppApi {
    VaultDir(): Promise<string>
    ListVaults(): Promise<VaultInfo[]>
    IsUnlocked(): Promise<boolean>
    CreateVault(name: string, password: string, confirm: string): Promise<void>
    OpenVault(file: string, password: string): Promise<void>
    GetCurrentVaultName(): Promise<string>
    DeleteVault(file: string): Promise<void>
    DeleteAccount(): Promise<void>
    ChangeMasterPassword(oldPassword: string, newPassword: string, confirm: string): Promise<void>
    Lock(): Promise<void>
    GetItems(): Promise<Item[]>
    CreateItem(input: Item): Promise<Item>
    UpdateItem(input: Item): Promise<void>
    DeleteItem(id: string): Promise<void>
    DeleteItems(ids: string[]): Promise<void>
    ListTrashed(): Promise<Item[]>
    RestoreTrashed(id: string): Promise<void>
    PurgeTrashed(ids: string[]): Promise<void>
    SetTrashDays(days: number): Promise<void>
    AddAttachment(itemId: string, name: string, dataB64: string): Promise<Attachment>
    ListAttachments(itemId: string): Promise<Attachment[]>
    GetAttachment(id: string): Promise<AttachmentPayload>
    DeleteAttachment(id: string): Promise<void>
    BackupNow(): Promise<string>
    ImportKeePassDB(dataB64: string, password: string): Promise<ImportResult>
    SetCategoryBatch(ids: string[], category: string): Promise<void>
    AddTagBatch(ids: string[], tag: string): Promise<void>
    SetFavoriteBatch(ids: string[], favorite: boolean): Promise<void>
    ExportSelectedCSV(ids: string[]): Promise<string>
    ExportSelectedJSON(ids: string[]): Promise<string>
    GeneratePassword(opts: PasswordOptions): Promise<string>
    GeneratePassphrase(words: number): Promise<string>
    CopyToClipboard(text: string): Promise<void>
    GetItemVersions(itemId: string): Promise<VersionEntry[]>
    RestoreVersion(versionId: string): Promise<Item>
    GetSettings(): Promise<Record<string, string>>
    SetSetting(key: string, value: string): Promise<void>
    SetAutolockMinutes(minutes: number): Promise<void>
    SetCloseToTray(enabled: boolean): Promise<void>
    SetQuickAccess(enabled: boolean): Promise<void>
    CloseQuickAccess(): Promise<void>
    GetSyncConfig(): Promise<Record<string, string>>
    SetSyncConfig(provider: string, remote: string): Promise<void>
    DisconnectSync(): Promise<void>
    SyncNow(): Promise<string>
    GetSyncStatus(): Promise<SyncStatus>
    DriveConnect(clientId: string, clientSecret: string): Promise<string>
    DriveDisconnect(): Promise<void>
    DriveSetupFolder(): Promise<string>
    InstallNativeHost(chromeExtID: string, firefoxAddonID: string): Promise<void>
    UninstallNativeHost(): Promise<void>
    GeneratePairingCode(): Promise<string>
    PrefetchFavicons(): Promise<void>
    ImportCSV(data: string, mapping: FieldMapping[]): Promise<ImportResult>
    ImportAutoCSV(data: string): Promise<ImportResult>
    ImportBitwardenJSON(data: string): Promise<ImportResult>
    ImportEncryptedTransfer(data: string, password: string): Promise<ImportResult>
    ImportCommit(items: Item[]): Promise<ImportResult>
    ExportCSV(): Promise<string>
    ExportJSON(): Promise<string>
    ExportEncryptedJSON(password: string): Promise<string>
    GenerateTOTPSetup(itemId: string): Promise<TOTPSetupInfo>
    GetTOTPCode(itemId: string): Promise<TOTPCode>
    GetTOTPCodeForSecret(secret: string): Promise<TOTPCode>
    ValidateTOTPSecret(secret: string): Promise<void>
    IngestTOTPURI(uri: string): Promise<string>
    AnalyzeVault(): Promise<HealthReport>
}
