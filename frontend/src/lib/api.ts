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
    vaultExists: (): Promise<boolean> => window.go.main.App.VaultExists(),
    createVault: (password: string, confirm: string): Promise<void> =>
        window.go.main.App.CreateVault(password, confirm),
    openVault: (password: string): Promise<void> => window.go.main.App.OpenVault(password),
    lock: (): Promise<void> => window.go.main.App.Lock(),
    changeMasterPassword: (oldPassword: string, newPassword: string, confirm: string): Promise<void> =>
        window.go.main.App.ChangeMasterPassword(oldPassword, newPassword, confirm),
    getItems: (): Promise<import('./types').Item[]> => window.go.main.App.GetItems(),
    createItem: (item: import('./types').Item): Promise<import('./types').Item> =>
        window.go.main.App.CreateItem(item),
    updateItem: (item: import('./types').Item): Promise<void> => window.go.main.App.UpdateItem(item),
    deleteItem: (id: string): Promise<void> => window.go.main.App.DeleteItem(id),
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
