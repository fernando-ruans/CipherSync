package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/trustelem/zxcvbn"
)

type ItemRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Score int    `json:"score"`
}

type DuplicateGroup struct {
	Password string    `json:"password"`
	Items    []ItemRef `json:"items"`
}

type HealthReport struct {
	TotalItems      int              `json:"totalItems"`
	TotalPasswords  int              `json:"totalPasswords"`
	TotalScore      int              `json:"totalScore"`
	WeakCount       int              `json:"weakCount"`
	DuplicateCount  int              `json:"duplicateCount"`
	OldCount        int              `json:"oldCount"`
	Missing2FA      int              `json:"missing2FA"`
	BreachedCount   int              `json:"breachedCount"`
	BreachCheckError bool            `json:"breachCheckError"`
	WeakItems       []ItemRef        `json:"weakItems"`
	OldItems        []ItemRef        `json:"oldItems"`
	Missing2FAItems []ItemRef        `json:"missing2FAItems"`
	BreachedItems   []ItemRef        `json:"breachedItems"`
	DuplicateGroups []DuplicateGroup `json:"duplicateGroups"`
}

var commonWeakPasswords = map[string]bool{
	"password": true, "123456": true, "12345678": true, "123456789": true, "qwerty": true,
	"abc123": true, "111111": true, "123123": true, "admin": true, "letmein": true,
	"password1": true, "iloveyou": true, "sunshine": true, "princess": true, "monkey": true,
	"1234567": true, "1234567890": true, "11111111": true, "000000": true, "welcome": true,
	"login": true, "senha": true, "1234": true, "senha123": true, "batman": true, "qwerty123": true,
	"football": true, "baseball": true, "dragon": true, "master": true, "passw0rd": true,
}

var lowerRe = regexp.MustCompile(`[a-z]`)
var upperRe = regexp.MustCompile(`[A-Z]`)
var digitRe = regexp.MustCompile(`[0-9]`)
var symbolRe = regexp.MustCompile(`[^a-zA-Z0-9]`)

func goPasswordScore(pw string) int {
	if pw == "" {
		return 0
	}
	if commonWeakPasswords[strings.ToLower(pw)] {
		return 0
	}
	// zxcvbn: real pattern/dictionary analysis (0-4)
	zx := zxcvbn.PasswordStrength(pw, nil).Score
	score := 0
	if len(pw) >= 8 {
		score++
	}
	if len(pw) >= 14 {
		score++
	}
	variety := 0
	for _, re := range []*regexp.Regexp{lowerRe, upperRe, digitRe, symbolRe} {
		if re.MatchString(pw) {
			variety++
		}
	}
	if variety >= 3 {
		score++
	}
	if variety == 4 && len(pw) >= 12 {
		score++
	}
	// zxcvbn catches dictionary/leaked patterns the heuristic misses
	if zx < score {
		score = zx
	}
	if score > 4 {
		score = 4
	}
	return score
}

// ---------- HIBP (k-anonymity) ----------

var breachCache = struct {
	sync.Mutex
	results map[string]int
}{results: map[string]int{}}

const maxBreachCache = 2000

// checkBreach queries the Have I Been Pwned range API. Only the first 5
// characters of the password's SHA-1 hash leave the machine.
func checkBreach(password string) (breached bool, count int, err error) {
	if password == "" {
		return false, 0, nil
	}
	sum := sha1.Sum([]byte(password))
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := full[:5], full[5:]

	breachCache.Lock()
	if c, ok := breachCache.results[full]; ok {
		breachCache.Unlock()
		return c > 0, c, nil
	}
	breachCache.Unlock()

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get("https://api.pwnedpasswords.com/range/" + prefix)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, 0, fmt.Errorf("HIBP respondeu com status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return false, 0, err
	}

	count = 0
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Split(strings.TrimSpace(line), ":")
		if len(parts) == 2 && strings.EqualFold(parts[0], suffix) {
			_, _ = fmt.Sscanf(parts[1], "%d", &count)
			break
		}
	}

	breachCache.Lock()
	breachCache.results[full] = count
	if len(breachCache.results) > maxBreachCache {
		breachCache.results = map[string]int{}
	}
	breachCache.Unlock()
	return count > 0, count, nil
}

func checkBreachesConcurrent(passwords []string, maxWorkers int) (map[string]int, bool) {
	results := map[string]int{}
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if len(passwords) == 0 {
		return results, false
	}
	uniq := map[string]bool{}
	for _, p := range passwords {
		if p != "" {
			uniq[p] = true
		}
	}
	if len(uniq) == 0 {
		return results, false
	}

	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	hadError := false
	for pw := range uniq {
		wg.Add(1)
		sem <- struct{}{}
		go func(p string) {
			defer wg.Done()
			defer func() { <-sem }()
			_, count, err := checkBreach(p)
			if err != nil {
				mu.Lock()
				hadError = true
				mu.Unlock()
				return
			}
			mu.Lock()
			results[p] = count
			mu.Unlock()
		}(pw)
	}
	wg.Wait()
	return results, hadError
}

// breachChecker is a hook so tests can avoid real network calls.
var breachChecker = checkBreachesConcurrent

// ---------- Report ----------

func analyzeVault(items []Item) HealthReport {
	report := HealthReport{
		TotalItems:   len(items),
		WeakItems:    []ItemRef{},
		OldItems:     []ItemRef{},
		Missing2FAItems: []ItemRef{},
		BreachedItems: []ItemRef{},
		DuplicateGroups: []DuplicateGroup{},
	}

	now := time.Now()
	oneYear := 365 * 24 * time.Hour

	byPassword := map[string][]ItemRef{}
	distinctPasswords := []string{}

	for _, it := range items {
		if it.Deleted {
			continue
		}
		if it.Type != TypeLogin {
			continue
		}
		if it.Password == "" {
			continue
		}
		report.TotalPasswords++
		ref := ItemRef{ID: it.ID, Title: it.Title, Score: goPasswordScore(it.Password)}

		if ref.Score < 2 {
			report.WeakCount++
			report.WeakItems = append(report.WeakItems, ref)
		}

		// duplicate tracking
		byPassword[it.Password] = append(byPassword[it.Password], ref)

		// old: password not changed in over a year
		changed := it.UpdatedAt
		if changed > 0 && now.Sub(time.UnixMilli(changed)) > oneYear {
			report.OldCount++
			report.OldItems = append(report.OldItems, ref)
		}

		// missing 2FA: real accounts (has URL) without TOTP
		if it.URL != "" && it.TotpSecret == "" {
			report.Missing2FA++
			report.Missing2FAItems = append(report.Missing2FAItems, ref)
		}

		distinctPasswords = append(distinctPasswords, it.Password)
	}

	// duplicates
	for pw, refs := range byPassword {
		if len(refs) >= 2 {
			group := DuplicateGroup{Password: pw, Items: refs}
			report.DuplicateGroups = append(report.DuplicateGroups, group)
			report.DuplicateCount += len(refs) - 1
		}
	}
	sort.Slice(report.DuplicateGroups, func(i, j int) bool {
		return len(report.DuplicateGroups[i].Items) > len(report.DuplicateGroups[j].Items)
	})

	// HIBP
	breachResults, breachErr := breachChecker(distinctPasswords, 4)
	report.BreachCheckError = breachErr
	for pw, refs := range byPassword {
		if count, ok := breachResults[pw]; ok && count > 0 {
			report.BreachedCount += len(refs)
			report.BreachedItems = append(report.BreachedItems, refs...)
		}
	}

	// score 0-100
	score := 100
	score -= report.WeakCount * 10
	score -= report.DuplicateCount * 5
	score -= report.OldCount * 5
	score -= report.Missing2FA * 4
	score -= report.BreachedCount * 20
	if score < 0 {
		score = 0
	}
	report.TotalScore = score

	return report
}
