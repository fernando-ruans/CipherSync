package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	gokeepasslib "github.com/tobischo/gokeepasslib/v3"
)

const transferMagic = "LKCIPH"

// ---------- Import ----------

func detectDelimiter(data string) rune {
	// detect ; (Excel BR) vs , by counting on the first non-empty line
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Count(line, ";") > strings.Count(line, ",") {
			return ';'
		}
		return ','
	}
	return ','
}

func parseCSVRows(data string) ([][]string, error) {
	data = strings.TrimPrefix(data, string(rune(0xFEFF)))
	r := csv.NewReader(strings.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.Comma = detectDelimiter(data)
	return r.ReadAll()
}

// parseCSV imports a generic CSV using an explicit column-to-field mapping.
func parseCSV(data string, mapping []FieldMapping) ([]Item, error) {
	rows, err := parseCSVRows(data)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return []Item{}, nil
	}
	return buildItemsFromRows(rows[1:], func(row []string) Item {
		it := Item{Type: TypeLogin}
		for _, m := range mapping {
			if m.Column < 0 || m.Column >= len(row) {
				continue
			}
			val := strings.TrimSpace(row[m.Column])
			switch m.Field {
			case "title":
				it.Title = val
			case "username":
				it.Username = val
			case "password":
				it.Password = val
			case "url":
				it.URL = val
			case "notes":
				it.Notes = val
			case "category":
				it.Category = val
			}
		}
		return it
	}), nil
}

// parseAutoCSV imports a CSV using header names to detect fields.
// Covers LastPass, 1Password and Bitwarden CSV exports.
func parseAutoCSV(data string) ([]Item, error) {
	rows, err := parseCSVRows(data)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return []Item{}, nil
	}
	colMap := map[string]int{}
	for i, h := range rows[0] {
		colMap[strings.ToLower(strings.TrimSpace(h))] = i
	}
	col := func(aliases ...string) int {
		for _, a := range aliases {
			if idx, ok := colMap[a]; ok {
				return idx
			}
		}
		return -1
	}
	titleCol := col("title", "name", "item name", "itemname")
	userCol := col("username", "user", "login", "login name", "email")
	passCol := col("password", "pass")
	urlCol := col("url", "website", "website url", "websiteurl", "site", "web site", "web page")
	notesCol := col("notes", "note", "extra", "comment", "comments", "notes - general")
	catCol := col("grouping", "group", "category", "folder", "group name")

	return buildItemsFromRows(rows[1:], func(row []string) Item {
		it := Item{Type: TypeLogin}
		get := func(idx int) string {
			if idx < 0 || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		it.Title = get(titleCol)
		it.Username = get(userCol)
		it.Password = get(passCol)
		it.URL = get(urlCol)
		it.Notes = get(notesCol)
		it.Category = get(catCol)
		if it.Title == "" && it.URL != "" {
			if d := extractDomain(it.URL); d != "" {
				it.Title = d
			}
		}
		return it
	}), nil
}

func buildItemsFromRows(rows [][]string, build func([]string) Item) []Item {
	items := []Item{}
	seen := map[string]bool{}
	for _, row := range rows {
		it := build(row)
		if it.Title == "" && it.Username == "" && it.Password == "" && it.Notes == "" && it.URL == "" {
			continue
		}
		key := strings.ToLower(it.Title + "|" + it.Username + "|" + it.Password)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, it)
	}
	return items
}

type bitwardenJSON struct {
	Encrypted bool `json:"encrypted"`
	Folders   []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"folders"`
	Items []struct {
		Name      string `json:"name"`
		FolderID  string `json:"folderId"`
		Favorite  bool   `json:"favorite"`
		Notes     string `json:"notes"`
		Type      int    `json:"type"`
		Login     *struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Uris     []struct {
				URI string `json:"uri"`
			} `json:"uris"`
		} `json:"login"`
		SecureNote *struct{} `json:"secureNote"`
		Card       *struct {
			CardholderName string `json:"cardholderName"`
			Brand          string `json:"brand"`
			Number         string `json:"number"`
			ExpMonth       string `json:"expMonth"`
			ExpYear        string `json:"expYear"`
			Code           string `json:"code"`
		} `json:"card"`
		Identity *struct {
			Title      string `json:"title"`
			FirstName  string `json:"firstName"`
			MiddleName string `json:"middleName"`
			LastName   string `json:"lastName"`
			Address1   string `json:"address1"`
			Address2   string `json:"address2"`
			City       string `json:"city"`
			State      string `json:"state"`
			PostalCode string `json:"postalCode"`
			Country    string `json:"country"`
			Phone      string `json:"phone"`
			Email      string `json:"email"`
		} `json:"identity"`
	} `json:"items"`
}

func parseBitwardenJSON(data string) ([]Item, error) {
	var bw bitwardenJSON
	if err := json.Unmarshal([]byte(data), &bw); err != nil {
		return nil, err
	}
	if bw.Encrypted {
		return nil, errors.New("vault is encrypted; export it unencrypted first")
	}
	folderNames := map[string]string{}
	for _, f := range bw.Folders {
		folderNames[f.ID] = f.Name
	}

	items := []Item{}
	seen := map[string]bool{}
	for _, src := range bw.Items {
		it := Item{Title: src.Name, Notes: src.Notes, Favorite: src.Favorite, Type: TypeLogin}
		it.Category = folderNames[src.FolderID]

		switch src.Type {
		case 0: // login
			if src.Login != nil {
				it.Username = src.Login.Username
				it.Password = src.Login.Password
				if len(src.Login.Uris) > 0 {
					it.URL = src.Login.Uris[0].URI
				}
			}
		case 1: // secure note
			it.Type = TypeNote
			it.Username = ""
			it.Password = ""
			it.URL = ""
		case 2: // card
			it.Type = TypeCreditCard
			it.Fields = map[string]string{}
			if src.Card != nil {
				it.Fields["cardholder"] = src.Card.CardholderName
				it.Fields["number"] = src.Card.Number
				it.Fields["brand"] = src.Card.Brand
				it.Fields["cvv"] = src.Card.Code
				exp := strings.TrimSpace(src.Card.ExpMonth)
				year := strings.TrimSpace(src.Card.ExpYear)
				if len(year) == 4 {
					year = year[2:]
				}
				if exp != "" || year != "" {
					it.Fields["expiry"] = exp + "/" + year
				}
			}
		case 3: // identity
			it.Type = TypeIdentity
			it.Fields = map[string]string{}
			if src.Identity != nil {
				fullName := strings.TrimSpace(strings.Join([]string{
					src.Identity.Title, src.Identity.FirstName, src.Identity.MiddleName, src.Identity.LastName,
				}, " "))
				it.Fields["fullName"] = fullName
				it.Fields["address"] = strings.TrimSpace(src.Identity.Address1 + " " + src.Identity.Address2)
				it.Fields["city"] = src.Identity.City
				it.Fields["state"] = src.Identity.State
				it.Fields["zip"] = src.Identity.PostalCode
				it.Fields["phone"] = src.Identity.Phone
				it.Fields["email"] = src.Identity.Email
			}
		}

		key := strings.ToLower(it.Title + "|" + it.Username + "|" + it.Password)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, it)
	}
	return items, nil
}

func parseKeePassDB(raw []byte, password string) ([]Item, error) {
	db := gokeepasslib.NewDatabase()
	db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	if err := gokeepasslib.NewDecoder(bytes.NewReader(raw)).Decode(db); err != nil {
		return nil, err
	}
	if db.Content == nil {
		return nil, errors.New("invalid KeePass database")
	}
	items := []Item{}
	seen := map[string]bool{}
	var walk func(groups []gokeepasslib.Group)
	walk = func(groups []gokeepasslib.Group) {
		for _, g := range groups {
			for _, e := range g.Entries {
				it := Item{Type: TypeLogin, Category: g.Name}
				it.Title = e.GetContent("Title")
				it.Username = e.GetContent("UserName")
				it.Password = e.GetPassword()
				it.URL = e.GetContent("URL")
				it.Notes = e.GetContent("Notes")
				if it.Title == "" && it.Username == "" && it.Password == "" && it.URL == "" && it.Notes == "" {
					continue
				}
				key := strings.ToLower(it.Title + "|" + it.Username + "|" + it.Password)
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, it)
			}
			walk(g.Groups)
		}
	}
	walk(db.Content.Root.Groups)
	return items, nil
}

func (v *Vault) importItems(items []Item) ImportResult {
	v.mu.Lock()
	defer v.mu.Unlock()
	res := ImportResult{}
	tx, err := v.db.Begin()
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	var pending []Item
	// batch accumulates accepted items so duplicate passkeys *within the same
	// import batch* are also rejected
	batch := append([]Item{}, v.items...)
	for _, it := range items {
		normalizeItem(&it)
		if it.Type == "" {
			it.Type = TypeLogin
		}
		// same validation as create/update so imports cannot bypass it
		if it.Passkey != nil {
			if err := validatePasskey("", it.Passkey, batch); err != nil {
				res.Errors = append(res.Errors, it.Title+": "+err.Error())
				continue
			}
		}
		// NOTE: same title+username with a DIFFERENT password is kept on
		// purpose (password variants), matching the parse-level dedupe
		if v.findDuplicate(it) {
			res.Skipped++
			continue
		}
		it.ID = uuid.NewString()
		it.CreatedAt = now
		it.UpdatedAt = now
		if err := v.persistItemTx(tx, it); err != nil {
			res.Errors = append(res.Errors, it.Title+": "+err.Error())
			continue
		}
		pending = append(pending, it)
		batch = append(batch, it)
		res.Created++
	}
	if err := tx.Commit(); err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res
	}
	v.items = append(v.items, pending...)
	v.sortItems()
	return res
}

func (v *Vault) findDuplicate(candidate Item) bool {
	for _, ex := range v.items {
		if ex.Deleted {
			continue
		}
		if strings.EqualFold(ex.Title, candidate.Title) &&
			strings.EqualFold(ex.Username, candidate.Username) {
			return true
		}
	}
	return false
}

// ---------- Export ----------

func exportCSV(items []Item) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"title", "username", "password", "url", "notes", "category", "type"})
	for _, it := range items {
		_ = w.Write([]string{
			it.Title, it.Username, it.Password, it.URL, it.Notes, it.Category, it.Type,
		})
	}
	w.Flush()
	return buf.String()
}

func exportJSON(items []Item) (string, error) {
	// strip passkey private key material but keep the reference fields
	sanitized := make([]Item, len(items))
	for i, it := range items {
		if it.Passkey != nil {
			pk := *it.Passkey
			pk.PrivateKey = ""
			it.Passkey = &pk
		}
		sanitized[i] = it
	}
	b, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// sealTransfer encrypts items for transfer between LockSync instances.
func sealTransfer(items []Item, password string) (string, error) {
	plain, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	params := newKDFParams()
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	salt, err := randomBytes(saltLen)
	if err != nil {
		return "", err
	}
	key := deriveKey(password, salt, params)
	enc, err := encrypt(key, plain)
	wipe(key)
	if err != nil {
		wipe(salt)
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString(transferMagic)
	buf.WriteByte(1)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(paramsJSON)))
	buf.Write(paramsJSON)
	buf.Write(salt)
	buf.Write(enc)
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// openTransfer decrypts a sealed transfer produced by sealTransfer.
func openTransfer(data, password string) ([]Item, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, errors.New("invalid transfer file")
	}
	r := bytes.NewReader(raw)
	magic := make([]byte, len(transferMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, errors.New("invalid transfer file")
	}
	// accept current and legacy (pre-rebrand) magic headers
	if string(magic) != transferMagic && string(magic) != "LKSYNC" {
		return nil, errors.New("invalid transfer file")
	}
	ver, err := r.ReadByte()
	if err != nil || ver != 1 {
		return nil, errors.New("unsupported transfer version")
	}
	var pLen uint32
	if err := binary.Read(r, binary.BigEndian, &pLen); err != nil {
		return nil, errors.New("invalid transfer file")
	}
	// kdfParams JSON is a few hundred bytes; a huge length means a
	// malformed/hostile file and would otherwise allocate unbounded memory.
	if pLen > 1<<20 {
		return nil, errors.New("invalid transfer file")
	}
	paramsJSON := make([]byte, pLen)
	if _, err := io.ReadFull(r, paramsJSON); err != nil {
		return nil, errors.New("invalid transfer file")
	}
	var params kdfParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, errors.New("invalid transfer file")
	}
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(r, salt); err != nil {
		return nil, errors.New("invalid transfer file")
	}
	enc, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	key := deriveKey(password, salt, params)
	plain, err := decrypt(key, enc)
	wipe(key)
	wipe(salt)
	if err != nil {
		return nil, ErrWrongPassword
	}
	var items []Item
	if err := json.Unmarshal(plain, &items); err != nil {
		return nil, err
	}
	return items, nil
}
