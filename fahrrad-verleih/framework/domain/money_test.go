package domain

import "testing"

func TestMoneyString(t *testing.T) {
	cases := map[Money]string{
		0:            "0,00 €",
		Euro(5):      "5,00 €",
		950:          "9,50 €",
		Euro(1240):   "1.240,00 €",
		Euro(123456): "123.456,00 €",
		-1205:        "-12,05 €",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("Money(%d) = %q, want %q", int64(m), got, want)
		}
	}
}

func TestParseMoney(t *testing.T) {
	ok := map[string]Money{"6": 600, "9.5": 950, "9,50": 950, "12,05": 1205, " 14 € ": 1400, "0.99": 99}
	for in, want := range ok {
		got, err := ParseMoney(in)
		if err != nil || got != want {
			t.Errorf("ParseMoney(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "1.234", "5.", "-3", ".5", "1,2,3"} {
		if _, err := ParseMoney(in); err == nil {
			t.Errorf("ParseMoney(%q) sollte fehlschlagen", in)
		}
	}
}
