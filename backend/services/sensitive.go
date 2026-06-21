package services

import (
	"clipboard-sync/models"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type SensitiveWordService struct {
	customWords  []string
	wordsUpdated time.Time
	cacheTTL     time.Duration
	mu           sync.RWMutex
}

const (
	SensitiveKeyCustom    = "sensitive:custom:words"
	SensitiveKeyPatterns  = "sensitive:patterns"
	SensitiveCacheTTL     = 5 * time.Second
	PhonePattern          = `1[3-9]\d{9}`
	IDCardPattern         = `[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`
	EmailPattern          = `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	BankCardPattern       = `[1-9]\d{14,18}`
)

var defaultPatterns = map[string]string{
	"phone":    PhonePattern,
	"idcard":   IDCardPattern,
	"email":    EmailPattern,
	"bankcard": BankCardPattern,
}

var compiledPatterns = map[string]*regexp.Regexp{}
var sharedSensitiveService *SensitiveWordService
var sharedOnce sync.Once

func NewSensitiveWordService() *SensitiveWordService {
	sharedOnce.Do(func() {
		sharedSensitiveService = &SensitiveWordService{
			cacheTTL: SensitiveCacheTTL,
		}
		for name, pattern := range defaultPatterns {
			compiledPatterns[name] = regexp.MustCompile(pattern)
		}
	})
	return sharedSensitiveService
}

func (s *SensitiveWordService) loadCustomWords() error {
	now := time.Now()
	s.mu.RLock()
	recent := now.Sub(s.wordsUpdated) < s.cacheTTL
	s.mu.RUnlock()

	if recent {
		return nil
	}

	words, err := models.RDB.SMembers(context.Background(), SensitiveKeyCustom).Result()
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.customWords = words
	s.wordsUpdated = now
	s.mu.Unlock()

	return nil
}

func (s *SensitiveWordService) FilterText(text string) (string, []string, error) {
	hitWords := []string{}
	result := text

	for name, re := range compiledPatterns {
		matches := re.FindAllString(result, -1)
		if len(matches) > 0 {
			for _, m := range matches {
				hitWords = append(hitWords, fmt.Sprintf("%s:%s", name, m))
			}
			result = re.ReplaceAllStringFunc(result, func(match string) string {
				return maskString(match)
			})
		}
	}

	if err := s.loadCustomWords(); err != nil {
		return result, hitWords, err
	}

	s.mu.RLock()
	customWords := make([]string, len(s.customWords))
	copy(customWords, s.customWords)
	s.mu.RUnlock()

	for _, word := range customWords {
		if word == "" {
			continue
		}
		if strings.Contains(result, word) {
			hitWords = append(hitWords, fmt.Sprintf("custom:%s", word))
			mask := strings.Repeat("*", len([]rune(word)))
			result = strings.ReplaceAll(result, word, mask)
		}
	}

	return result, hitWords, nil
}

func (s *SensitiveWordService) AddCustomWord(word string) error {
	if strings.TrimSpace(word) == "" {
		return nil
	}
	err := models.RDB.SAdd(context.Background(), SensitiveKeyCustom, strings.TrimSpace(word)).Err()
	if err == nil {
		s.mu.Lock()
		s.wordsUpdated = time.Time{}
		s.mu.Unlock()
	}
	return err
}

func (s *SensitiveWordService) RemoveCustomWord(word string) error {
	err := models.RDB.SRem(context.Background(), SensitiveKeyCustom, word).Err()
	if err == nil {
		s.mu.Lock()
		s.wordsUpdated = time.Time{}
		s.mu.Unlock()
	}
	return err
}

func (s *SensitiveWordService) ListCustomWords() ([]string, error) {
	return models.RDB.SMembers(context.Background(), SensitiveKeyCustom).Result()
}

func maskString(s string) string {
	runes := []rune(s)
	n := len(runes)
	if n <= 2 {
		return strings.Repeat("*", n)
	}
	keepStart := n / 4
	keepEnd := n / 4
	if keepStart < 1 {
		keepStart = 1
	}
	if keepEnd < 1 {
		keepEnd = 1
	}
	maskLen := n - keepStart - keepEnd
	if maskLen < 1 {
		maskLen = 1
		keepStart = 1
		keepEnd = 0
	}
	return string(runes[:keepStart]) + strings.Repeat("*", maskLen) + string(runes[n-keepEnd:])
}

func (s *SensitiveWordService) RefreshCache() {
	s.mu.Lock()
	s.wordsUpdated = time.Time{}
	s.mu.Unlock()
}
