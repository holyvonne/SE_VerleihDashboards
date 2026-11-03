package fahrrad

import (
	"fmt"
	"strings"
	"time"

	"fahrradverleih/framework/domain"
)

// Config ist die Konfiguration des Fahrrad-Plugins (Spezifikation 4.7).
// Preise und Grenzen werden hier festgelegt, nicht im Framework.
type Config struct {
	Rates       map[string]domain.Money // Stundensatz je Fahrradtyp
	MinDuration time.Duration
	MaxDuration time.Duration
}

// MaxRate ist die Obergrenze eines Stundensatzes, um Tippfehler abzufangen.
const MaxRate = domain.Money(100_00)

// DefaultConfig liefert die bestätigten Standardwerte (A-1, A-3).
func DefaultConfig() Config {
	return Config{
		Rates: map[string]domain.Money{
			Citybike:     domain.Euro(5),
			Mountainbike: domain.Euro(8),
			EBike:        domain.Euro(12),
		},
		MinDuration: time.Hour,
		MaxDuration: 30 * 24 * time.Hour,
	}
}

// WithRates liefert eine Kopie mit überschriebenen Stundensätzen.
func (c Config) WithRates(rates map[string]domain.Money) Config {
	merged := make(map[string]domain.Money, len(c.Rates))
	for k, v := range c.Rates {
		merged[k] = v
	}
	for k, v := range rates {
		merged[k] = v
	}
	c.Rates = merged
	return c
}

// Validate prüft die Konfiguration: Jeder Typ hat einen Satz über 0 € und höchstens 100 €.
func (c Config) Validate() error {
	for _, cat := range categories {
		rate, ok := c.Rates[cat.Key]
		if !ok {
			return fmt.Errorf("kein Stundensatz für %s", cat.Label)
		}
		if err := checkRate(cat.Key, rate); err != nil {
			return err
		}
	}
	for key := range c.Rates {
		if !knownCategory(key) {
			return fmt.Errorf("unbekannter Fahrradtyp %q (erlaubt: %s)", key, categoryKeys())
		}
	}
	if c.MinDuration < 0 || c.MaxDuration <= 0 || c.MinDuration >= c.MaxDuration {
		return fmt.Errorf("Mindestdauer muss kleiner als die Höchstdauer sein")
	}
	return nil
}

func checkRate(key string, rate domain.Money) error {
	if rate <= 0 || rate > MaxRate {
		return fmt.Errorf("Stundensatz für %s muss über 0 € und höchstens %s liegen", key, MaxRate)
	}
	return nil
}

// ParseRates liest Stundensätze wie „citybike=6,mountainbike=9.50,ebike=14“ (Flag -tarife, AK-18).
func ParseRates(spec string) (map[string]domain.Money, error) {
	rates := map[string]domain.Money{}
	if strings.TrimSpace(spec) == "" {
		return rates, nil
	}
	for _, part := range strings.Split(spec, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		key = strings.ToLower(strings.TrimSpace(key))
		if !ok || key == "" {
			return nil, fmt.Errorf("ungültige Angabe %q, erwartet typ=betrag", part)
		}
		if !knownCategory(key) {
			return nil, fmt.Errorf("unbekannter Fahrradtyp %q (erlaubt: %s)", key, categoryKeys())
		}
		rate, err := domain.ParseMoney(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		if err := checkRate(key, rate); err != nil {
			return nil, err
		}
		rates[key] = rate
	}
	return rates, nil
}

func knownCategory(key string) bool {
	for _, c := range categories {
		if c.Key == key {
			return true
		}
	}
	return false
}

func categoryKeys() string {
	keys := make([]string, len(categories))
	for i, c := range categories {
		keys[i] = c.Key
	}
	return strings.Join(keys, ", ")
}
