package convo

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bot-jual/internal/atl"
)

var amountRegex = regexp.MustCompile(`\d+(?:[.,]?\d+)?`)

func filterByQuery(items []atl.PriceListItem, query, provider string, full bool) []atl.PriceListItem {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if query == "" && provider == "" {
		res := make([]atl.PriceListItem, len(items))
		copy(res, items)
		sort.Slice(res, func(i, j int) bool {
			left := strings.ToLower(res[i].Category)
			right := strings.ToLower(res[j].Category)
			if left == right {
				return res[i].Price < res[j].Price
			}
			return left < right
		})
		if full {
			return res
		}
		return topN(res, 10)
	}

	query = strings.TrimSpace(strings.ToLower(query))
	tokens := tokenizeQuery(query)

	var scored []scoredItem
	for _, item := range items {
		score := matchScore(item, tokens, provider)
		if score > 0 {
			scored = append(scored, scoredItem{Item: item, Score: score})
		}
	}

	if len(scored) == 0 && provider != "" {
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Provider), provider) {
				scored = append(scored, scoredItem{Item: item, Score: 1})
			}
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Item.Price < scored[j].Item.Price
		}
		return scored[i].Score > scored[j].Score
	})

	top := make([]atl.PriceListItem, 0, len(scored))
	for _, sc := range scored {
		top = append(top, sc.Item)
	}
	if full {
		return top
	}
	return topN(top, 10)
}

func filterByBudget(items []atl.PriceListItem, budget int64) []atl.PriceListItem {
	var res []atl.PriceListItem
	for _, item := range items {
		if item.Price <= float64(budget) && strings.EqualFold(item.Status, "available") {
			res = append(res, item)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Price < res[j].Price
	})
	return topN(res, 10)
}

func formatPriceList(items []atl.PriceListItem, full bool) string {
	categoryMap, order := groupByCategory(items)
	if len(order) == 0 {
		return "Belum ada produk yang cocok."
	}

	var builder strings.Builder
	if full {
		builder.WriteString("Daftar produk lengkap:\n")
	} else {
		builder.WriteString("Daftar produk:\n")
	}

	for _, category := range order {
		builder.WriteString("- ")
		builder.WriteString(category)
		builder.WriteString(":\n")

		entries := categoryMap[category]
		limit := len(entries)
		if !full && limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			item := entries[i]
			builder.WriteString("  - ")
			builder.WriteString(fmt.Sprintf("%s (%s) - Rp%.0f [%s]", item.Name, item.Code, item.Price, strings.ToUpper(item.Status)))
			builder.WriteString("\n")
		}
		if !full && len(entries) > limit {
			builder.WriteString("  - ...\n")
		}
	}

	return strings.TrimSpace(builder.String())
}

func formatCatalogSummary(items []atl.PriceListItem) string {
	categoryMap, order := groupByCategory(items)
	if len(order) == 0 {
		return "Belum ada produk yang tersedia."
	}

	var builder strings.Builder
	builder.WriteString("Daftar produk lengkap:\n")
	for _, category := range order {
		builder.WriteString(strings.ToUpper(category))
		builder.WriteString(":\n")

		entries := categoryMap[category]
		limit := len(entries)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			item := entries[i]
			builder.WriteString("  - ")
			builder.WriteString(fmt.Sprintf("%s (%s) - Rp%.0f [%s]", item.Name, item.Code, item.Price, strings.ToUpper(item.Status)))
			builder.WriteString("\n")
		}
		if len(entries) > limit {
			builder.WriteString("  - ...\n")
		}
	}
	builder.WriteString("\nKetik nama kategori atau provider untuk daftar lebih rinci.")

	return strings.TrimSpace(builder.String())
}

func matchScore(item atl.PriceListItem, tokens []string, provider string) int {
	name := strings.ToLower(item.Name)
	code := strings.ToLower(item.Code)
	category := strings.ToLower(item.Category)
	itemProvider := strings.ToLower(item.Provider)

	if provider != "" && !strings.Contains(itemProvider, provider) {
		return 0
	}

	score := 0
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(name, token) {
			score += 4
		}
		if strings.Contains(code, token) {
			score += 5
		}
		if strings.Contains(category, token) {
			score += 3
		}
		if strings.Contains(itemProvider, token) {
			score += 3
		}
	}
	return score
}

func parseAmount(text string) (int64, error) {
	if text == "" {
		return 0, fmt.Errorf("empty amount")
	}
	text = strings.ToLower(strings.TrimSpace(text))
	matches := amountRegex.FindAllString(text, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("no numeric value")
	}

	value := matches[0]
	value = strings.ReplaceAll(value, ".", "")
	value = strings.ReplaceAll(value, ",", "")

	num, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	if strings.Contains(text, "k") && num < 1000 {
		num *= 1000
	}
	if strings.Contains(text, "m") && num < 1000000 {
		num *= 1000000
	}
	return num, nil
}

type scoredItem struct {
	Item  atl.PriceListItem
	Score int
}

func topN(items []atl.PriceListItem, n int) []atl.PriceListItem {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func groupByCategory(items []atl.PriceListItem) (map[string][]atl.PriceListItem, []string) {
	grouped := map[string][]atl.PriceListItem{}
	order := []string{}
	for _, item := range items {
		category := strings.TrimSpace(item.Category)
		if category == "" {
			category = "Lainnya"
		}
		if _, ok := grouped[category]; !ok {
			order = append(order, category)
		}
		grouped[category] = append(grouped[category], item)
	}
	for _, categoryItems := range grouped {
		sort.Slice(categoryItems, func(i, j int) bool {
			return categoryItems[i].Price < categoryItems[j].Price
		})
	}
	return grouped, order
}

func tokenizeQuery(query string) []string {
	if query == "" {
		return nil
	}
	query = strings.ReplaceAll(query, ".", " ")
	query = strings.ReplaceAll(query, ",", " ")
	rawTokens := strings.Fields(query)
	expanded := make([]string, 0, len(rawTokens)*2)
	for _, token := range rawTokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" {
			continue
		}
		expanded = append(expanded, token)
		if strings.ContainsAny(token, "0123456789") && strings.ContainsAny(token, "abcdefghijklmnopqrstuvwxyz") {
			builder := strings.Builder{}
			for _, r := range token {
				if r >= '0' && r <= '9' {
					builder.WriteRune(r)
				}
			}
			if builder.Len() > 0 {
				expanded = append(expanded, builder.String())
			}
		}
	}
	return expanded
}
