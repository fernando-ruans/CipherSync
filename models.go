package main

const (
	TypeLogin      = "login"
	TypeNote       = "note"
	TypeCreditCard = "credit_card"
	TypeIdentity   = "identity"
)

type Item struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Username  string            `json:"username"`
	Password  string            `json:"password"`
	URL       string            `json:"url"`
	Notes     string            `json:"notes"`
	Category  string            `json:"category"`
	Tags      []string          `json:"tags"`
	Fields    map[string]string `json:"fields"`
	Favorite  bool              `json:"favorite"`
	CreatedAt int64             `json:"createdAt"`
	UpdatedAt int64             `json:"updatedAt"`
}

type PasswordOptions struct {
	Length           int  `json:"length"`
	UseUpper         bool `json:"useUpper"`
	UseLower         bool `json:"useLower"`
	UseDigits        bool `json:"useDigits"`
	UseSymbols       bool `json:"useSymbols"`
	ExcludeAmbiguous bool `json:"excludeAmbiguous"`
}

type VersionEntry struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Item      Item   `json:"item"`
}

type FieldMapping struct {
	Column int    `json:"column"`
	Field  string `json:"field"`
}

type ImportResult struct {
	Created int      `json:"created"`
	Skipped int      `json:"skipped"`
	Preview []Item   `json:"preview"`
	Errors  []string `json:"errors"`
}

type VaultInfo struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	LastOpened int64  `json:"lastOpened"`
}
