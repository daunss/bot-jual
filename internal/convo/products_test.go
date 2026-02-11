package convo

import (
	"testing"

	"bot-jual/internal/atl"
)

func TestFilterByQueryPrefersAmount(t *testing.T) {
	items := []atl.PriceListItem{
		{
			Code:     "TSEL10",
			Name:     "Pulsa Telkomsel 10k",
			Category: "Pulsa",
			Provider: "Telkomsel",
			Nominal:  "10000",
			Price:    10000,
			Status:   "available",
		},
		{
			Code:     "TSEL20",
			Name:     "Pulsa Telkomsel 20k",
			Category: "Pulsa",
			Provider: "Telkomsel",
			Nominal:  "20000",
			Price:    20000,
			Status:   "available",
		},
	}

	matches := filterByQuery(items, "pulsa telkomsel 20k", "telkomsel", false)
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if matches[0].Code != "TSEL20" {
		t.Fatalf("expected TSEL20 first, got %s", matches[0].Code)
	}
}

func TestRefineMatchesByAmountUsesNominal(t *testing.T) {
	items := []atl.PriceListItem{
		{Code: "A", Name: "Item A", Nominal: "5000", Price: 7000},
		{Code: "B", Name: "Item B", Nominal: "10000", Price: 7000},
	}

	res := refineMatchesByAmount(items, 10000)
	if len(res) == 0 {
		t.Fatal("expected results")
	}
	if res[0].Code != "B" {
		t.Fatalf("expected B first, got %s", res[0].Code)
	}
}
