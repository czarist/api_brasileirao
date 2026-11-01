package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
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

// seasonPlan is one catalog season, independent of which adapter publishes it.
type seasonPlan struct {
	code string
	year int
}

// router sends each competition to the adapter that owns its code.
// FMF stays on www.fmf.com.br. Each portal and UIKit client stays on its own host.
// None of them uses SOURCE_BASE_URL. FAF stays on www.futeboldealagoas.net.
// FBF stays on www.fbf.org.br. FCF stays on fcf.com.br.
// FPF-PR stays on federacaopr.com.br. FGF-RS stays on www.fgf.com.br.
type router struct {
	cbf    *cbf.Source
	fmf    *fmf.Source
	portal *portal.Source
	uikit  *uikit.Source
	fafal  *fafal.Source
	fbf    *fbf.Source
	fcf    *fcf.Source
	fpfpr  *fpfpr.Source
	fgfrs  *fgfrs.Source
}

func newRouter(cbfClient *cbf.Client, fmfClient *fmf.Client, portalSource *portal.Source, uikitSource *uikit.Source, fafalSource *fafal.Source, fbfSource *fbf.Source, fcfSource *fcf.Source, fpfprSource *fpfpr.Source, fgfrsSource *fgfrs.Source) router {
	return router{cbf: cbf.NewSource(cbfClient), fmf: fmf.NewSource(fmfClient), portal: portalSource, uikit: uikitSource, fafal: fafalSource, fbf: fbfSource, fcf: fcfSource, fpfpr: fpfprSource, fgfrs: fgfrsSource}
}

func (r router) Reset() {
	r.cbf.Reset()
	r.fmf.Reset()
	r.portal.Reset()
	r.uikit.Reset()
	r.fafal.Reset()
	r.fbf.Reset()
	r.fcf.Reset()
	r.fpfpr.Reset()
	r.fgfrs.Reset()
}

func (r router) Plan(ctx context.Context, codes []string) ([]seasonPlan, error) {
	cbfCodes, fmfCodes, portalCodes, uikitCodes, fafalCodes, fbfCodes, fcfCodes, fpfprCodes, fgfrsCodes := splitCodes(codes)
	plans := make([]seasonPlan, 0, len(codes))
	if len(cbfCodes) > 0 {
		got, err := r.cbf.Plan(ctx, cbfCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Competition.Code, year: plan.Year})
		}
	}
	if len(fmfCodes) > 0 {
		got, err := r.fmf.Plan(ctx, fmfCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Competition.Code, year: plan.Year})
		}
	}
	if len(portalCodes) > 0 {
		got, err := r.portal.Plan(ctx, portalCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Code, year: plan.Year})
		}
	}
	if len(uikitCodes) > 0 {
		got, err := r.uikit.Plan(ctx, uikitCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Code, year: plan.Year})
		}
	}
	if len(fafalCodes) > 0 {
		got, err := r.fafal.Plan(ctx, fafalCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Code, year: plan.Year})
		}
	}
	if len(fbfCodes) > 0 {
		got, err := r.fbf.Plan(ctx, fbfCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Code, year: plan.Year})
		}
	}
	if len(fcfCodes) > 0 {
		got, err := r.fcf.Plan(ctx, fcfCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Code, year: plan.Year})
		}
	}
	if len(fpfprCodes) > 0 {
		got, err := r.fpfpr.Plan(ctx, fpfprCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Code, year: plan.Year})
		}
	}
	if len(fgfrsCodes) > 0 {
		got, err := r.fgfrs.Plan(ctx, fgfrsCodes)
		if err != nil {
			return nil, err
		}
		for _, plan := range got {
			plans = append(plans, seasonPlan{code: plan.Code, year: plan.Year})
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].code != plans[j].code {
			return plans[i].code < plans[j].code
		}
		return plans[i].year < plans[j].year
	})
	return plans, nil
}

func (r router) Fetch(ctx context.Context, competition string, year int, collected time.Time) (domain.Snapshot, error) {
	if _, ok := fmf.Lookup(competition); ok {
		return r.fmf.Fetch(ctx, competition, year, collected)
	}
	if _, _, ok := portal.Lookup(competition); ok {
		return r.portal.Fetch(ctx, competition, year, collected)
	}
	if _, _, ok := uikit.Lookup(competition); ok {
		return r.uikit.Fetch(ctx, competition, year, collected)
	}
	if _, ok := fafal.Lookup(competition); ok {
		return r.fafal.Fetch(ctx, competition, year, collected)
	}
	if _, ok := fbf.Lookup(competition); ok {
		return r.fbf.Fetch(ctx, competition, year, collected)
	}
	if _, ok := fcf.Lookup(competition); ok {
		return r.fcf.Fetch(ctx, competition, year, collected)
	}
	if _, ok := fpfpr.Lookup(competition); ok {
		return r.fpfpr.Fetch(ctx, competition, year, collected)
	}
	if _, ok := fgfrs.Lookup(competition); ok {
		return r.fgfrs.Fetch(ctx, competition, year, collected)
	}
	return r.cbf.Fetch(ctx, competition, year, collected)
}

func splitCodes(codes []string) (cbfCodes, fmfCodes, portalCodes, uikitCodes, fafalCodes, fbfCodes, fcfCodes, fpfprCodes, fgfrsCodes []string) {
	for _, code := range codes {
		if _, ok := fmf.Lookup(code); ok {
			fmfCodes = append(fmfCodes, code)
			continue
		}
		if _, _, ok := portal.Lookup(code); ok {
			portalCodes = append(portalCodes, code)
			continue
		}
		if _, _, ok := uikit.Lookup(code); ok {
			uikitCodes = append(uikitCodes, code)
			continue
		}
		if _, ok := fafal.Lookup(code); ok {
			fafalCodes = append(fafalCodes, code)
			continue
		}
		if _, ok := fbf.Lookup(code); ok {
			fbfCodes = append(fbfCodes, code)
			continue
		}
		if _, ok := fcf.Lookup(code); ok {
			fcfCodes = append(fcfCodes, code)
			continue
		}
		if _, ok := fpfpr.Lookup(code); ok {
			fpfprCodes = append(fpfprCodes, code)
			continue
		}
		if _, ok := fgfrs.Lookup(code); ok {
			fgfrsCodes = append(fgfrsCodes, code)
			continue
		}
		cbfCodes = append(cbfCodes, code)
	}
	return cbfCodes, fmfCodes, portalCodes, uikitCodes, fafalCodes, fbfCodes, fcfCodes, fpfprCodes, fgfrsCodes
}

func limitsFor(code string) (domain.Limits, bool) {
	if comp, ok := cbf.Lookup(code); ok {
		return comp.Limits(), true
	}
	if comp, ok := fmf.Lookup(code); ok {
		return comp.Limits(), true
	}
	if _, division, ok := portal.Lookup(code); ok {
		return division.Limits(), true
	}
	if _, division, ok := uikit.Lookup(code); ok {
		return division.Limits(), true
	}
	if _, ok := fafal.Lookup(code); ok {
		return fafal.Limits(), true
	}
	if _, ok := fbf.Lookup(code); ok {
		return fbf.Limits(), true
	}
	if _, ok := fcf.Lookup(code); ok {
		return fcf.Limits(), true
	}
	if _, ok := fpfpr.Lookup(code); ok {
		return fpfpr.Limits(), true
	}
	if _, ok := fgfrs.Lookup(code); ok {
		return fgfrs.Limits(), true
	}
	return domain.Limits{}, false
}

func newFMFClient(timeout, interval time.Duration, maxBody int64, retries int) (*fmf.Client, error) {
	client, err := fmf.NewClient(fmf.ClientOptions{
		Timeout:     timeout,
		MinInterval: interval,
		MaxBody:     maxBody,
		Retries:     retries,
	})
	if err != nil {
		return nil, fmt.Errorf("fmf client: %w", err)
	}
	return client, nil
}

func newPortalSource(timeout, interval time.Duration, maxBody int64, retries int) (*portal.Source, error) {
	sites := portal.Sites()
	clients := make([]*portal.Client, 0, len(sites))
	for _, site := range sites {
		client, err := portal.NewClient(site, portal.ClientOptions{
			Timeout:     timeout,
			MinInterval: interval,
			MaxBody:     maxBody,
			Retries:     retries,
		})
		if err != nil {
			return nil, fmt.Errorf("portal client %s: %w", site.Host, err)
		}
		clients = append(clients, client)
	}
	return portal.NewSource(clients...), nil
}

func newUikitSource(timeout, interval time.Duration, maxBody int64, retries int) (*uikit.Source, error) {
	sites := uikit.Sites()
	clients := make([]*uikit.Client, 0, len(sites))
	for _, site := range sites {
		client, err := uikit.NewClient(site, uikit.ClientOptions{
			Timeout:     timeout,
			MinInterval: interval,
			MaxBody:     maxBody,
			Retries:     retries,
		})
		if err != nil {
			return nil, fmt.Errorf("uikit client %s: %w", site.Host, err)
		}
		clients = append(clients, client)
	}
	return uikit.NewSource(clients...), nil
}

func newFafalSource(timeout, interval time.Duration, maxBody int64, retries int) (*fafal.Source, error) {
	client, err := fafal.NewClient(fafal.ClientOptions{
		Timeout:     timeout,
		MinInterval: interval,
		MaxBody:     maxBody,
		Retries:     retries,
	})
	if err != nil {
		return nil, fmt.Errorf("fafal client: %w", err)
	}
	return fafal.NewSource(client), nil
}

func newFBFSource(timeout, interval time.Duration, maxBody int64, retries int) (*fbf.Source, error) {
	client, err := fbf.NewClient(fbf.ClientOptions{
		Timeout:     timeout,
		MinInterval: interval,
		MaxBody:     maxBody,
		Retries:     retries,
	})
	if err != nil {
		return nil, fmt.Errorf("fbf client: %w", err)
	}
	return fbf.NewSource(client), nil
}

func newFCFSource(timeout, interval time.Duration, maxBody int64, retries int) (*fcf.Source, error) {
	client, err := fcf.NewClient(fcf.ClientOptions{
		Timeout:     timeout,
		MinInterval: interval,
		MaxBody:     maxBody,
		Retries:     retries,
	})
	if err != nil {
		return nil, fmt.Errorf("fcf client: %w", err)
	}
	return fcf.NewSource(client), nil
}

func newFPFPRSource(timeout, interval time.Duration, maxBody int64, retries int) (*fpfpr.Source, error) {
	client, err := fpfpr.NewClient(fpfpr.ClientOptions{
		Timeout:     timeout,
		MinInterval: interval,
		MaxBody:     maxBody,
		Retries:     retries,
	})
	if err != nil {
		return nil, fmt.Errorf("fpfpr client: %w", err)
	}
	return fpfpr.NewSource(client), nil
}

func newFGFRSSource(timeout, interval time.Duration, maxBody int64, retries int) (*fgfrs.Source, error) {
	client, err := fgfrs.NewClient(fgfrs.ClientOptions{
		Timeout:     timeout,
		MinInterval: interval,
		MaxBody:     maxBody,
		Retries:     retries,
	})
	if err != nil {
		return nil, fmt.Errorf("fgfrs client: %w", err)
	}
	return fgfrs.NewSource(client), nil
}
