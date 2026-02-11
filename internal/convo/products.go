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
	if amount, err := parseAmount(query); err == nil && amount > 0 {
		top = refineMatchesByAmount(top, amount)
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

	// Filter out common stop words that add no search value
	significantTokens := filterStopWords(tokens)
	if len(significantTokens) == 0 {
		significantTokens = tokens // fallback to all tokens
	}

	score := 0
	matched := 0
	for _, token := range significantTokens {
		if token == "" {
			continue
		}
		tokenScore := 0
		if strings.Contains(name, token) {
			tokenScore += 4
		}
		if strings.Contains(code, token) {
			tokenScore += 5
		}
		if strings.Contains(category, token) {
			tokenScore += 3
		}
		if strings.Contains(itemProvider, token) {
			tokenScore += 3
		}
		if tokenScore > 0 {
			matched++
		}
		score += tokenScore
	}

	// Require at least half of significant tokens to match (min 1)
	minRequired := (len(significantTokens) + 1) / 2
	if minRequired < 1 {
		minRequired = 1
	}
	if matched < minRequired {
		return 0
	}

	return score
}

// filterStopWords removes common Indonesian and English stop words from query tokens.
func filterStopWords(tokens []string) []string {
	stopWords := map[string]bool{
		"yang": true, "dan": true, "di": true, "ke": true, "dari": true,
		"ada": true, "ini": true, "itu": true, "untuk": true, "dengan": true,
		"saya": true, "mau": true, "bisa": true, "jual": true, "cari": true,
		"halo": true, "hai": true, "hi": true, "hey": true, "bang": true,
		"mas": true, "kak": true, "min": true, "gan": true, "bro": true,
		"tolong": true, "dong": true, "ya": true, "nih": true, "deh": true,
		"list": true, "harga": true, "kirim": true, "kirimkan": true,
		"dibawah": true, "diatas": true, "sekitar": true,
		"the": true, "is": true, "a": true, "an": true, "of": true,
		"ribu": true, "rb": true, "juta": true, "jt": true,
	}
	var result []string
	for _, t := range tokens {
		if !stopWords[t] {
			result = append(result, t)
		}
	}
	return result
}

func parseAmount(text string) (int64, error) {
	if text == "" {
		return 0, fmt.Errorf("empty amount")
	}
	text = strings.ToLower(strings.TrimSpace(text))

	// Match number followed by optional suffix (k, rb, ribu, jt, juta, m)
	// This avoids false matches like 'k' in 'kirim' or 'm' in 'mobile'
	re := regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(k|rb|ribu|jt|juta|m)?(?:\s|$|[^a-z])`)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		// Fallback: just find any number
		basic := amountRegex.FindString(text)
		if basic == "" {
			return 0, fmt.Errorf("no numeric value")
		}
		basic = strings.ReplaceAll(basic, ".", "")
		basic = strings.ReplaceAll(basic, ",", "")
		return strconv.ParseInt(basic, 10, 64)
	}

	numStr := matches[1]
	suffix := matches[2]
	numStr = strings.ReplaceAll(numStr, ".", "")
	numStr = strings.ReplaceAll(numStr, ",", "")

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	switch suffix {
	case "k", "rb", "ribu":
		num *= 1000
	case "jt", "juta", "m":
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

func refineMatchesByAmount(items []atl.PriceListItem, amount int64) []atl.PriceListItem {
	if len(items) <= 1 || amount <= 0 {
		return items
	}
	res := make([]atl.PriceListItem, len(items))
	copy(res, items)
	sort.SliceStable(res, func(i, j int) bool {
		left := amountDiff(res[i], amount)
		right := amountDiff(res[j], amount)
		if left == right {
			return res[i].Price < res[j].Price
		}
		return left < right
	})
	return res
}

func amountDiff(item atl.PriceListItem, amount int64) int64 {
	best := absInt64(int64(item.Price) - amount)
	if nominal := parseNominalAmount(item.Nominal); nominal > 0 {
		diff := absInt64(nominal - amount)
		if diff < best {
			best = diff
		}
	}
	return best
}

func parseNominalAmount(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	value = strings.ReplaceAll(value, ".", "")
	value = strings.ReplaceAll(value, ",", "")
	clean := ""
	for _, r := range value {
		if r >= '0' && r <= '9' {
			clean += string(r)
		}
	}
	if clean == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func absInt64(val int64) int64 {
	if val < 0 {
		return -val
	}
	return val
}
