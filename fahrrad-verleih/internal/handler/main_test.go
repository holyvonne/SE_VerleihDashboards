package handler

import (
	"io"
	"log"
	"os"
	"testing"
)

// TestMain schaltet das Request-Log (NF-10) ab, damit die Testausgabe lesbar bleibt.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}
