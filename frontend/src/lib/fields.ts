import type {ItemType} from './types'

export interface FieldDef {
    key: string
    label: string
    secret?: boolean
    placeholder?: string
    card?: boolean
    mask?: boolean
}

export const ITEM_TYPES: {value: ItemType; label: string}[] = [
    {value: 'login', label: 'Login'},
    {value: 'note', label: 'Nota segura'},
    {value: 'credit_card', label: 'Cartão de crédito'},
    {value: 'identity', label: 'Identidade'},
    {value: 'passkey', label: 'Passkey'},
]

export const TYPE_FIELDS: Record<ItemType, FieldDef[]> = {
    login: [],
    note: [],
    passkey: [],
    credit_card: [
        {key: 'cardholder', label: 'Nome no cartão'},
        {key: 'number', label: 'Número do cartão', card: true, placeholder: '1234 5678 9012 3456'},
        {key: 'expiry', label: 'Validade (MM/AA)', mask: true, placeholder: '12/28'},
        {key: 'cvv', label: 'CVV', secret: true, mask: true},
        {key: 'brand', label: 'Bandeira', placeholder: 'Visa, Mastercard...'},
        {key: 'pin', label: 'PIN', secret: true, mask: true},
    ],
    identity: [
        {key: 'fullName', label: 'Nome completo'},
        {key: 'documentNumber', label: 'CPF / RG'},
        {key: 'email', label: 'E-mail'},
        {key: 'phone', label: 'Telefone'},
        {key: 'address', label: 'Endereço'},
        {key: 'city', label: 'Cidade'},
        {key: 'state', label: 'Estado'},
        {key: 'zip', label: 'CEP'},
    ],
}

export const FIELD_LABELS: Record<string, string> = {
    title: 'Título',
    username: 'Usuário',
    password: 'Senha',
    url: 'URL',
    notes: 'Notas',
    category: 'Categoria',
}
