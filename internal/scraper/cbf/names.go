package cbf

import "strings"

// knownShortNames maps a folded club name to the abbreviation used by the API.
var knownShortNames = map[string]string{
	"flamengo":             "FLA",
	"palmeiras":            "PAL",
	"corinthians":          "COR",
	"sao paulo":            "SAO",
	"santos":               "SAN",
	"santos fc":            "SAN",
	"gremio":               "GRE",
	"internacional":        "INT",
	"fluminense":           "FLU",
	"botafogo":             "BOT",
	"vasco da gama":        "VAS",
	"vasco da gama saf":    "VAS",
	"atletico mineiro":     "CAM",
	"cruzeiro":             "CRU",
	"bahia":                "BAH",
	"athletico paranaense": "CAP",
	"atletico paranaense":  "CAP",
	"coritiba":             "CFC",
	"coritiba saf":         "CFC",
	"chapecoense":          "CHA",
	"mirassol":             "MIR",
	"red bull bragantino":  "RBB",
	"bragantino":           "RBB",
	"vitoria":              "VIT",
	"remo":                 "REM",
}

// ShortName returns a stable abbreviation for a club name.
func ShortName(name string) string {
	key := fold(name)
	if short, ok := knownShortNames[key]; ok {
		return short
	}
	parts := strings.Fields(key)
	if len(parts) == 0 {
		return ""
	}
	token := parts[0]
	if len(token) > 3 {
		token = token[:3]
	}
	return strings.ToUpper(token)
}

func fold(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch r {
		case 'á', 'à', 'â', 'ã', 'ä':
			b.WriteByte('a')
		case 'é', 'è', 'ê', 'ë':
			b.WriteByte('e')
		case 'í', 'ì', 'î', 'ï':
			b.WriteByte('i')
		case 'ó', 'ò', 'ô', 'õ', 'ö':
			b.WriteByte('o')
		case 'ú', 'ù', 'û', 'ü':
			b.WriteByte('u')
		case 'ç':
			b.WriteByte('c')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
