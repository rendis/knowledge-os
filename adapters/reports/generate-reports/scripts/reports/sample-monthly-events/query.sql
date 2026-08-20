--standardSQL
SELECT
  CONCAT(SUBSTR(event_date, 1, 4), '-', SUBSTR(event_date, 5, 2)) AS event_month,
  geo.country AS country,
  event_name,
  COUNT(*) AS event_count
FROM `example-analytics.example_dataset.events_*`
WHERE _TABLE_SUFFIX >= @start_suffix
  AND _TABLE_SUFFIX < @end_suffix
GROUP BY event_month, country, event_name
ORDER BY event_month, country, event_name
