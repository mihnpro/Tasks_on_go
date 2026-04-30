// Package reporter генерирует Markdown-отчёт по результатам бенчмарка
// и сохраняет его на диск — каждый запуск оставляет постоянный артефакт.
package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cache-comparison/internal/loadgen"
	"cache-comparison/internal/metrics"
)

const reportsDir = "reports"

// ReportConfig содержит параметры бенчмарка, включаемые в заголовок отчёта.
type ReportConfig struct {
	Duration      time.Duration
	Concurrency   int
	TotalKeys     int
	CacheTTL      time.Duration
	FlushInterval time.Duration // только для Write-Back
}

// Generate записывает Markdown-отчёт в reports/report_<timestamp>.md
// и возвращает путь к созданному файлу.
func Generate(reports []metrics.Report, cfg ReportConfig) (string, error) {
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return "", fmt.Errorf("создание папки reports: %w", err)
	}

	filename := filepath.Join(reportsDir,
		fmt.Sprintf("report_%s.md", time.Now().Format("2006-01-02_15-04-05")))

	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("создание файла отчёта: %w", err)
	}
	defer f.Close()

	w := &strings.Builder{}
	writeHeader(w)
	writeConfig(w, cfg)
	writeResultsTable(w, reports)
	writeScenarioAnalysis(w, reports)
	writeConclusions(w)
	writeFooter(w)

	if _, err = f.WriteString(w.String()); err != nil {
		return "", fmt.Errorf("запись отчёта: %w", err)
	}
	return filename, nil
}

// ── Разделы ──────────────────────────────────────────────────────────────────

func writeHeader(w *strings.Builder) {
	w.WriteString("# Отчёт по сравнению стратегий кеширования\n\n")
	fmt.Fprintf(w, "**Дата и время:** %s\n\n", time.Now().Format("02.01.2006 15:04:05"))
	w.WriteString("Сравнение трёх стратегий кеширования в трёх сценариях нагрузки.\n\n")
	w.WriteString("---\n\n")
}

func writeConfig(w *strings.Builder, cfg ReportConfig) {
	w.WriteString("## Конфигурация теста\n\n")
	w.WriteString("| Параметр | Значение |\n")
	w.WriteString("|----------|----------|\n")
	fmt.Fprintf(w, "| Длительность сценария         | `%s` |\n", cfg.Duration)
	fmt.Fprintf(w, "| Параллельных воркеров         | `%d` |\n", cfg.Concurrency)
	fmt.Fprintf(w, "| Размер ключевого пространства | `%d` |\n", cfg.TotalKeys)
	fmt.Fprintf(w, "| TTL кеша                      | `%s` |\n", cfg.CacheTTL)
	fmt.Fprintf(w, "| Интервал сброса Write-Back    | `%s` |\n", cfg.FlushInterval)
	w.WriteString("\n### Сценарии нагрузки\n\n")
	w.WriteString("| Название | Чтений | Записей |\n")
	w.WriteString("|----------|--------|---------|\n")
	for _, sc := range loadgen.Scenarios {
		read := int(sc.ReadRatio * 100)
		fmt.Fprintf(w, "| `%s` | %d%% | %d%% |\n", sc.Name, read, 100-read)
	}
	w.WriteString("\n---\n\n")
}

func writeResultsTable(w *strings.Builder, reports []metrics.Report) {
	w.WriteString("## Результаты\n\n")
	w.WriteString("| Стратегия | Сценарий | Пропускная способность (req/s) | Средняя задержка (мс) | P99 задержка (мс) | Обращений в БД | Hit Rate кеша | Всего операций |\n")
	w.WriteString("|-----------|----------|-------------------------------|----------------------|-------------------|----------------|---------------|----------------|\n")
	for _, r := range reports {
		fmt.Fprintf(w, "| %s | %s | %.0f | %.2f | %.2f | %d | %.1f%% | %d |\n",
			r.Strategy, r.Scenario,
			r.Throughput, r.AvgLatencyMs, r.P99LatencyMs,
			r.DBCalls, r.CacheHitRate, r.TotalOps,
		)
	}
	w.WriteString("\n---\n\n")
}

func writeScenarioAnalysis(w *strings.Builder, reports []metrics.Report) {
	w.WriteString("## Анализ по сценариям\n\n")

	grouped := make(map[string][]metrics.Report)
	for _, r := range reports {
		grouped[r.Scenario] = append(grouped[r.Scenario], r)
	}

	for _, sc := range loadgen.Scenarios {
		reps, ok := grouped[sc.Name]
		if !ok {
			continue
		}

		fmt.Fprintf(w, "### %s\n\n", sc.Name)

		type winner struct {
			metric string
			name   string
			value  string
		}
		winners := []winner{
			{
				metric: "Лучшая пропускная способность",
				name:   bestBy(reps, func(a, b metrics.Report) bool { return a.Throughput > b.Throughput }).Strategy,
				value:  fmt.Sprintf("%.0f req/s", maxFloat(reps, func(r metrics.Report) float64 { return r.Throughput })),
			},
			{
				metric: "Наименьшая средняя задержка",
				name:   bestBy(reps, func(a, b metrics.Report) bool { return a.AvgLatencyMs < b.AvgLatencyMs }).Strategy,
				value:  fmt.Sprintf("%.2f мс", minFloat(reps, func(r metrics.Report) float64 { return r.AvgLatencyMs })),
			},
			{
				metric: "Наименьшая P99 задержка",
				name:   bestBy(reps, func(a, b metrics.Report) bool { return a.P99LatencyMs < b.P99LatencyMs }).Strategy,
				value:  fmt.Sprintf("%.2f мс", minFloat(reps, func(r metrics.Report) float64 { return r.P99LatencyMs })),
			},
			{
				metric: "Меньше всего обращений в БД",
				name:   bestBy(reps, func(a, b metrics.Report) bool { return a.DBCalls < b.DBCalls }).Strategy,
				value:  fmt.Sprintf("%d", minInt(reps, func(r metrics.Report) int64 { return r.DBCalls })),
			},
			{
				metric: "Лучший hit rate кеша",
				name:   bestBy(reps, func(a, b metrics.Report) bool { return a.CacheHitRate > b.CacheHitRate }).Strategy,
				value:  fmt.Sprintf("%.1f%%", maxFloat(reps, func(r metrics.Report) float64 { return r.CacheHitRate })),
			},
		}

		w.WriteString("| Метрика | Победитель | Значение |\n")
		w.WriteString("|---------|------------|----------|\n")
		for _, win := range winners {
			fmt.Fprintf(w, "| %s | **%s** | %s |\n", win.metric, win.name, win.value)
		}

		// Детальная таблица по стратегиям, отсортированная по пропускной способности.
		w.WriteString("\n**Детали:**\n\n")
		w.WriteString("| Стратегия | Пропускная способность | Средняя задержка | P99 задержка | Обращений в БД | Hit Rate |\n")
		w.WriteString("|-----------|------------------------|------------------|--------------|----------------|----------|\n")
		sorted := make([]metrics.Report, len(reps))
		copy(sorted, reps)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Throughput > sorted[j].Throughput })
		for _, r := range sorted {
			fmt.Fprintf(w, "| %s | %.0f req/s | %.2f мс | %.2f мс | %d | %.1f%% |\n",
				r.Strategy, r.Throughput, r.AvgLatencyMs, r.P99LatencyMs, r.DBCalls, r.CacheHitRate)
		}
		w.WriteString("\n")
	}
	w.WriteString("---\n\n")
}

func writeConclusions(w *strings.Builder) {
	w.WriteString("## Выводы\n\n")
	w.WriteString("### Когда использовать каждую стратегию\n\n")

	w.WriteString("#### Lazy Loading / Cache-Aside\n")
	w.WriteString("- **Лучше всего подходит для:** нагрузки с преобладанием чтений, где допустима небольшая устаревшесть данных.\n")
	w.WriteString("- Запись идёт напрямую в БД, кеш-запись инвалидируется.\n")
	w.WriteString("- Кеш заполняется только по запросу (при первом промахе) — холодный старт даёт повышенную нагрузку на БД.\n")
	w.WriteString("- Минимальные накладные расходы на запись — нет лишнего обращения к кешу при обновлении.\n\n")

	w.WriteString("#### Write-Through\n")
	w.WriteString("- **Лучше всего подходит для:** смешанной нагрузки или нагрузки с преобладанием чтений, где важна строгая согласованность.\n")
	w.WriteString("- Каждая запись синхронно обновляет и БД, и кеш — чтения после записи всегда быстрые.\n")
	w.WriteString("- Более высокая задержка записи по сравнению с Lazy Loading (два обращения за одну операцию).\n")
	w.WriteString("- Лучший общий hit rate кеша после фазы прогрева.\n\n")

	w.WriteString("#### Write-Back\n")
	w.WriteString("- **Лучше всего подходит для:** нагрузки с преобладанием записей, где важна максимальная пропускная способность.\n")
	w.WriteString("- Запись попадает только в кеш; в БД данные сбрасываются асинхронно фоновым воркером.\n")
	w.WriteString("- Наименьшая задержка записи и наибольшая пропускная способность из трёх стратегий.\n")
	w.WriteString("- Риск: данные, записанные с момента последнего сброса, теряются при аварийном завершении процесса.\n\n")

	w.WriteString("### Сводная таблица\n\n")
	w.WriteString("| Стратегия | Производительность чтения | Производительность записи | Согласованность | Сложность реализации |\n")
	w.WriteString("|-----------|--------------------------|--------------------------|-----------------|----------------------|\n")
	w.WriteString("| Lazy Loading  | ✅ Хорошая | ✅ Хорошая       | ⚠️ Возможна устаревшесть | Низкая  |\n")
	w.WriteString("| Write-Through | ✅ Хорошая | ⚠️ Доп. запись в БД | ✅ Строгая       | Средняя |\n")
	w.WriteString("| Write-Back    | ✅ Хорошая | 🚀 Наилучшая     | ⚠️ Риск потери данных | Высокая |\n")
	w.WriteString("\n---\n\n")
}

func writeFooter(w *strings.Builder) {
	fmt.Fprintf(w, "*Отчёт сгенерирован автоматически бенчмарком cache-comparison — %s*\n",
		time.Now().Format("02.01.2006 15:04:05"))
}

// ── Вспомогательные функции ───────────────────────────────────────────────────

func bestBy(reports []metrics.Report, better func(a, b metrics.Report) bool) metrics.Report {
	best := reports[0]
	for _, r := range reports[1:] {
		if better(r, best) {
			best = r
		}
	}
	return best
}

func maxFloat(reports []metrics.Report, val func(metrics.Report) float64) float64 {
	m := val(reports[0])
	for _, r := range reports[1:] {
		if v := val(r); v > m {
			m = v
		}
	}
	return m
}

func minFloat(reports []metrics.Report, val func(metrics.Report) float64) float64 {
	m := val(reports[0])
	for _, r := range reports[1:] {
		if v := val(r); v < m {
			m = v
		}
	}
	return m
}

func minInt(reports []metrics.Report, val func(metrics.Report) int64) int64 {
	m := val(reports[0])
	for _, r := range reports[1:] {
		if v := val(r); v < m {
			m = v
		}
	}
	return m
}
