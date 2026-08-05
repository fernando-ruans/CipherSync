import type {AppApi} from './types'

declare global {
    interface Window {
        go: {
            main: {
                App: AppApi
            }
        }
    }
}

export const api = {
    listVaults: (): Promise<import('./types').VaultInfo[]> => window.go.main.App.ListVaults(),
    createVault: (name: string, password: string, confirm: string): Promise<void> =>
        window.go.main.App.CreateVault(name, password, confirm),
    openVault: (file: string, password: string): Promise<void> =>
        window.go.main.App.OpenVault(file, password),
    getCurrentVaultName: (): Promise<string> => window.go.main.App.GetCurrentVaultName(),
    deleteVault: (file: string): Promise<void> => window.go.main.App.DeleteVault(file),
    deleteAccount: (): Promise<void> => window.go.main.App.DeleteAccount(),
    lock: (): Promise<void> => window.go.main.App.Lock(),
    changeMasterPassword: (oldPassword: string, newPassword: string, confirm: string): Promise<void> =>
        window.go.main.App.ChangeMasterPassword(oldPassword, newPassword, confirm),
    getItems: (): Promise<import('./types').Item[]> => window.go.main.App.GetItems(),
    createItem: (item: import('./types').Item): Promise<import('./types').Item> =>
        window.go.main.App.CreateItem(item),
    updateItem: (item: import('./types').Item): Promise<void> => window.go.main.App.UpdateItem(item),
    deleteItem: (id: string): Promise<void> => window.go.main.App.DeleteItem(id),
    deleteItems: (ids: string[]): Promise<void> => window.go.main.App.DeleteItems(ids),
    setCategoryBatch: (ids: string[], category: string): Promise<void> =>
        window.go.main.App.SetCategoryBatch(ids, category),
    addTagBatch: (ids: string[], tag: string): Promise<void> => window.go.main.App.AddTagBatch(ids, tag),
    setFavoriteBatch: (ids: string[], favorite: boolean): Promise<void> =>
        window.go.main.App.SetFavoriteBatch(ids, favorite),
    exportSelectedCSV: (ids: string[]): Promise<string> => window.go.main.App.ExportSelectedCSV(ids),
    exportSelectedJSON: (ids: string[]): Promise<string> => window.go.main.App.ExportSelectedJSON(ids),
    generatePassword: (opts: import('./types').PasswordOptions): Promise<string> =>
        window.go.main.App.GeneratePassword(opts),
    generatePassphrase: (words: number): Promise<string> => window.go.main.App.GeneratePassphrase(words),
    copy: (text: string): Promise<void> => window.go.main.App.CopyToClipboard(text),
    getItemVersions: (itemId: string): Promise<import('./types').VersionEntry[]> =>
        window.go.main.App.GetItemVersions(itemId),
    restoreVersion: (versionId: string): Promise<import('./types').Item> =>
        window.go.main.App.RestoreVersion(versionId),
    getSettings: (): Promise<Record<string, string>> => window.go.main.App.GetSettings(),
    setSetting: (key: string, value: string): Promise<void> => window.go.main.App.SetSetting(key, value),
    setAutolockMinutes: (minutes: number): Promise<void> => window.go.main.App.SetAutolockMinutes(minutes),
    prefetchFavicons: (): Promise<void> => window.go.main.App.PrefetchFavicons(),
    importCSV: (data: string, mapping: import('./types').FieldMapping[]): Promise<import('./types').ImportResult> =>
        window.go.main.App.ImportCSV(data, mapping),
    importAutoCSV: (data: string): Promise<import('./types').ImportResult> =>
        window.go.main.App.ImportAutoCSV(data),
    importBitwardenJSON: (data: string): Promise<import('./types').ImportResult> =>
        window.go.main.App.ImportBitwardenJSON(data),
    importEncryptedTransfer: (data: string, password: string): Promise<import('./types').ImportResult> =>
        window.go.main.App.ImportEncryptedTransfer(data, password),
    importCommit: (items: import('./types').Item[]): Promise<import('./types').ImportResult> =>
        window.go.main.App.ImportCommit(items),
    exportCSV: (): Promise<string> => window.go.main.App.ExportCSV(),
    exportJSON: (): Promise<string> => window.go.main.App.ExportJSON(),
    exportEncryptedJSON: (password: string): Promise<string> => window.go.main.App.ExportEncryptedJSON(password),
}

export async function errorMessage(e: unknown): Promise<string> {
    if (e instanceof Error) return e.message
    return String(e)
}
