// Package catalog is the list of competition codes the process accepts.
package catalog

import (
	"fmt"
	"strings"

	"github.com/czarist/api_brasileirao/internal/scraper/cbf"
	"github.com/czarist/api_brasileirao/internal/scraper/fafal"
	"github.com/czarist/api_brasileirao/internal/scraper/fbf"
	"github.com/czarist/api_brasileirao/internal/scraper/fcf"
	"github.com/czarist/api_brasileirao/internal/scraper/fgfrs"
	"github.com/czarist/api_brasileirao/internal/scraper/fmf"
	"github.com/czarist/api_brasileirao/internal/scraper/fpfpr"
	"github.com/czarist/api_brasileirao/internal/scraper/portal"
	"github.com/czarist/api_brasileirao/internal/scraper/uikit"
)

// Parse accepts "all" or a comma-separated list. "all" is every supported
// competition, in source order: the CBF catalog, the FMF catalog, then the
// state portals.
func Parse(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "all" {
		codes, err := cbf.ParseCodes("all")
		if err != nil {
			return nil, err
		}
		codes = append(codes, fmf.Codes()...)
		codes = append(codes, portal.Codes()...)
		codes = append(codes, uikit.Codes()...)
		codes = append(codes, fafal.Codes()...)
		codes = append(codes, fbf.Codes()...)
		codes = append(codes, fcf.Codes()...)
		codes = append(codes, fpfpr.Codes()...)
		return append(codes, fgfrs.Codes()...), nil
	}
	parts := strings.Split(spec, ",")
	codes := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := cbf.Lookup(part); !ok {
			if _, ok = fmf.Lookup(part); !ok {
				if _, _, ok = portal.Lookup(part); !ok {
					if _, _, ok = uikit.Lookup(part); !ok {
						if _, ok = fafal.Lookup(part); !ok {
							if _, ok = fbf.Lookup(part); !ok {
								if _, ok = fcf.Lookup(part); !ok {
									if _, ok = fpfpr.Lookup(part); !ok {
										if _, ok = fgfrs.Lookup(part); !ok {
											return nil, fmt.Errorf("unsupported competition %s", part)
										}
									}
								}
							}
						}
					}
				}
			}
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		codes = append(codes, part)
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("SYNC_COMPETITION is empty")
	}
	return codes, nil
}
