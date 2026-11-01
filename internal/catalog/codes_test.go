package catalog

import "testing"

func TestParseAllKeepsTheCBFOrderAndAppendsMineiro(t *testing.T) {
	codes, err := Parse("all")
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 24 || codes[0] != "brasileirao-serie-a" || codes[4] != "copa-do-brasil" || codes[7] != "mineiro-segunda-divisao" || codes[8] != "candangao-serie-a" || codes[15] != "pernambucano-a3" || codes[16] != "maranhense-serie-a" || codes[19] != "mato-grossense-segunda-divisao" || codes[20] != "alagoano-serie-a" || codes[21] != "alagoano-serie-b" || codes[22] != "baiano-serie-a" || codes[23] != "baiano-serie-b" {
		t.Fatalf("codes = %v", codes)
	}
}

func TestParseAcceptsAMineiroCodeAndRejectsAnUnknownOne(t *testing.T) {
	codes, err := Parse("mineiro-modulo-i, brasileirao-serie-a, mineiro-modulo-i")
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 2 || codes[0] != "mineiro-modulo-i" || codes[1] != "brasileirao-serie-a" {
		t.Fatalf("codes = %v", codes)
	}
	if _, err := Parse("serie-b"); err == nil {
		t.Fatal("unknown code was accepted")
	}
}
