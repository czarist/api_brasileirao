package catalog

import "testing"

func TestParseAllKeepsTheCBFOrderAndAppendsMineiro(t *testing.T) {
	codes, err := Parse("all")
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 36 || codes[0] != "brasileirao-serie-a" || codes[4] != "copa-do-brasil" || codes[7] != "mineiro-segunda-divisao" || codes[8] != "candangao-serie-a" || codes[15] != "pernambucano-a3" || codes[16] != "maranhense-serie-a" || codes[19] != "mato-grossense-segunda-divisao" || codes[20] != "alagoano-serie-a" || codes[21] != "alagoano-serie-b" || codes[22] != "baiano-serie-a" || codes[23] != "baiano-serie-b" || codes[24] != "catarinense-serie-a" || codes[26] != "catarinense-serie-c" || codes[27] != "paranaense" || codes[28] != "paranaense-segundona" || codes[29] != "paranaense-terceirona" || codes[30] != "gauchao" || codes[31] != "gauchao-serie-a2" || codes[32] != "gauchao-serie-b" || codes[33] != "sergipano-a1" || codes[34] != "sergipano-a2" || codes[35] != "sergipano-a3" {
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
