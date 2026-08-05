package main

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

const (
	lowerChars  = "abcdefghijklmnopqrstuvwxyz"
	upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars  = "0123456789"
	symbolChars = "!@#$%^&*()-_=+[]{};:,.?/|"
	ambiguous   = "il1Lo0O"
)

func cryptoRandInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func generatePassword(opts PasswordOptions) (string, error) {
	if opts.Length <= 0 {
		return "", errors.New("length must be greater than zero")
	}
	if opts.Length > 256 {
		return "", errors.New("length too large")
	}
	if !opts.UseUpper && !opts.UseLower && !opts.UseDigits && !opts.UseSymbols {
		return "", errors.New("at least one character type must be selected")
	}

	charSets := []string{}
	if opts.UseLower {
		charSets = append(charSets, lowerChars)
	}
	if opts.UseUpper {
		charSets = append(charSets, upperChars)
	}
	if opts.UseDigits {
		charSets = append(charSets, digitChars)
	}
	if opts.UseSymbols {
		charSets = append(charSets, symbolChars)
	}

	var pool strings.Builder
	for _, s := range charSets {
		if opts.ExcludeAmbiguous {
			s = stripChars(s, ambiguous)
		}
		if len(s) == 0 {
			continue
		}
		pool.WriteString(s)
	}
	if pool.Len() == 0 {
		return "", errors.New("character pool is empty after exclusions")
	}
	poolStr := pool.String()

	var sb strings.Builder
	for i := 0; i < opts.Length; i++ {
		idx, err := cryptoRandInt(len(poolStr))
		if err != nil {
			return "", err
		}
		sb.WriteByte(poolStr[idx])
	}
	return sb.String(), nil
}

func stripChars(s, chars string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(chars, r) {
			return -1
		}
		return r
	}, s)
}

// generatePassphrase builds a word-based passphrase from an embedded wordlist.
func generatePassphrase(words int) (string, error) {
	if words <= 0 {
		return "", errors.New("word count must be greater than zero")
	}
	if words > 16 {
		return "", errors.New("word count too large")
	}
	parts := make([]string, 0, words)
	for i := 0; i < words; i++ {
		idx, err := cryptoRandInt(len(wordlist))
		if err != nil {
			return "", err
		}
		parts = append(parts, wordlist[idx])
	}
	return strings.Join(parts, "-"), nil
}
