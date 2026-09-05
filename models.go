package main

const (
	TypeLogin      = "login"
	TypeNote       = "note"
	TypeCreditCard = "credit_card"
	TypeIdentity   = "identity"
	TypePasskey    = "passkey"
)

type Item struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Username   string            `json:"username"`
	Password   string            `json:"password"`
	URL        string            `json:"url"`
	Notes      string            `json:"notes"`
	Category   string            `json:"category"`
	Tags       []string          `json:"tags"`
	Fields     map[string]string `json:"fields"`
	TotpSecret string            `json:"totpSecret"`
	Passkey    *PasskeyData      `json:"passkey,omitempty"`
	Favorite   bool              `json:"favorite"`
	Deleted    bool              `json:"deleted"`
	DeletedAt  int64             `json:"deletedAt"`
	CreatedAt  int64             `json:"createdAt"`
	UpdatedAt  int64             `json:"updatedAt"`
}

// PasskeyData stores a passkey credential reference (Scope A: inventory only,
// not a FIDO2 authenticator). Lives inside the encrypted Item JSON blob.
type PasskeyData struct {
	CredentialID string   `json:"credentialId"`
	RpID         string   `json:"rpId"`
	RpName       string   `json:"rpName"`
	UserHandle   string   `json:"userHandle"`
	Username     string   `json:"username"`
	DisplayName  string   `json:"displayName"`
	PrivateKey   string   `json:"privateKey"`
	PublicKey    string   `json:"publicKey"`
	CoseAlg      int      `json:"coseAlg"`
	Transports   []string `json:"transports"`
	AAGUID       string   `json:"aaguid"`
	BackupState  string   `json:"backupState"`
}

type Attachment struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	AddedAt int64  `json:"addedAt"`
}

type AttachmentPayload struct {
	Name string `json:"name"`
	Data string `json:"data"`
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

type TOTPSetupInfo struct {
	Secret     string `json:"secret"`
	QR         string `json:"qr"`
	OtpauthURL string `json:"otpauthURL"`
}

type TOTPCode struct {
	Code             string `json:"code"`
	SecondsRemaining int    `json:"secondsRemaining"`
}
