package lake

import "brevis.sh/brevis/sdk"

// DailyRevenue é a tabela gold.daily_revenue: receita e volume por dia e por
// host, recalculada a cada run do `metrics`. A tabela só anexa, como todo
// destino Iceberg do SDK hoje, então cada run grava uma versão com
// `computed_at`, e quem lê pega a mais recente. O ouro também tem história.
var DailyRevenue = sdk.Schema{
	{Name: "day", Type: sdk.TypeDate, Required: true},
	{Name: "host", Type: sdk.TypeString, Required: true},
	{Name: "events", Type: sdk.TypeInt64, Required: true},
	{Name: "quantity", Type: sdk.TypeInt64},
	{Name: "revenue", Type: sdk.TypeNumeric},
	{Name: "computed_at", Type: sdk.TypeTimestamp, Required: true},
}

// LatestDailyRevenue é a consulta da API: a versão mais recente do ouro, por
// dia e por host, lida pelo metadata.json corrente da tabela gold.
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

// BronzeToDailyRevenue é a consulta do `metrics`: do bronze, por dia e host.
func BronzeToDailyRevenue(metadata string) string {
	// sum() de inteiros no DuckDB é HUGEINT, que o JSON do CLI carrega como
	// texto; o cast para BIGINT devolve um número, que é o que a coluna
	// int64 do Iceberg aceita. O decimal vai como texto de propósito.
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
