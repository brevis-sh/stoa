package lake

import "brevis.sh/brevis/sdk"

// DailyRevenue is the gold.daily_revenue table: revenue and volume per day
// and per host, recomputed on every run of `metrics`. The table only appends,
// as every Iceberg destination in the SDK does today, so each run writes a
// version stamped with `computed_at` and whoever reads takes the most recent
// one. The gold has history too.
var DailyRevenue = sdk.Schema{
	{Name: "day", Type: sdk.TypeDate, Required: true},
	{Name: "host", Type: sdk.TypeString, Required: true},
	{Name: "events", Type: sdk.TypeInt64, Required: true},
	{Name: "quantity", Type: sdk.TypeInt64},
	{Name: "revenue", Type: sdk.TypeNumeric},
	{Name: "computed_at", Type: sdk.TypeTimestamp, Required: true},
}

// LatestDailyRevenue is the API's query: the most recent version of the gold,
// per day and per host, read by the gold table's current metadata.json.
func LatestDailyRevenue(metadata, from, to string) string {
	where := ""
	if from != "" {
		where += " AND day >= DATE " + sqlQuote(from)
	}
	if to != "" {
		where += " AND day <= DATE " + sqlQuote(to)
	}
	return `WITH gold AS (SELECT * FROM iceberg_scan(` + sqlQuote(metadata) + `))
SELECT day::VARCHAR AS day, host, events, quantity, revenue::VARCHAR AS revenue, computed_at::VARCHAR AS computed_at
FROM gold WHERE computed_at = (SELECT max(computed_at) FROM gold)` + where + `
ORDER BY day, host`
}

// BronzeToDailyRevenue is the `metrics` query: from the bronze, per day and host.
func BronzeToDailyRevenue(metadata string) string {
	// sum() over integers is HUGEINT in DuckDB, and the CLI's JSON carries it
	// as text; the cast to BIGINT gives back a number, which is what the
	// Iceberg int64 column takes. The decimal goes as text on purpose.
	return `SELECT CAST(occurred_at AS DATE)::VARCHAR AS day, host, count(*)::BIGINT AS events, sum(quantity)::BIGINT AS quantity,
sum(amount)::VARCHAR AS revenue
FROM iceberg_scan(` + sqlQuote(metadata) + `) GROUP BY 1, 2 ORDER BY 1, 2`
}

func sqlQuote(s string) string {
	out := "'"
	for _, r := range s {
		if r == '\'' {
			out += "'"
		}
		out += string(r)
	}
	return out + "'"
}
