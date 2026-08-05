export type ItemType = 'login' | 'note' | 'credit_card' | 'identity'

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
    favorite: boolean
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

export type Phase = 'loading' | 'setup' | 'unlock' | 'main'

export interface AppApi {
    VaultPath(): Promise<string>
    VaultExists(): Promise<boolean>
    IsUnlocked(): Promise<boolean>
    CreateVault(password: string, confirm: string): Promise<void>
    OpenVault(password: string): Promise<void>
    ChangeMasterPassword(oldPassword: string, newPassword: string, confirm: string): Promise<void>
    Lock(): Promise<void>
    GetItems(): Promise<Item[]>
    CreateItem(input: Item): Promise<Item>
    UpdateItem(input: Item): Promise<void>
    DeleteItem(id: string): Promise<void>
    GeneratePassword(opts: PasswordOptions): Promise<string>
    GeneratePassphrase(words: number): Promise<string>
    CopyToClipboard(text: string): Promise<void>
    GetItemVersions(itemId: string): Promise<VersionEntry[]>
    RestoreVersion(versionId: string): Promise<Item>
    GetSettings(): Promise<Record<string, string>>
    SetSetting(key: string, value: string): Promise<void>
    SetAutolockMinutes(minutes: number): Promise<void>
    PrefetchFavicons(): Promise<void>
    ImportCSV(data: string, mapping: FieldMapping[]): Promise<ImportResult>
    ImportAutoCSV(data: string): Promise<ImportResult>
    ImportBitwardenJSON(data: string): Promise<ImportResult>
    ImportEncryptedTransfer(data: string, password: string): Promise<ImportResult>
    ImportCommit(items: Item[]): Promise<ImportResult>
    ExportCSV(): Promise<string>
    ExportJSON(): Promise<string>
    ExportEncryptedJSON(password: string): Promise<string>
}
