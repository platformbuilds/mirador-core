#!/usr/bin/env python3
import json

def generate_aggregate_postman_items():
    # List of aggregate functions from the OpenAPI
    functions = [
        "sum", "avg", "count", "min", "max", "median", "quantile", "topk", "bottomk", "distinct",
        "histogram", "outliers_iqr", "outliersk", "stddev", "stdvar", "mad", "zscore", "mode",
        "skewness", "kurtosis", "cov", "range", "delta", "idelta", "increase", "irate", "rate",
        "geomean", "harmean", "trimean", "iqr", "percentile", "entropy", "mode_multi", "count_values", "corr"
    ]

    postman_items = []
    for func in functions:
        item = {
            "name": f"POST /metrics/query/aggregate/{func}",
            "request": {
                "method": "POST",
                "header": [{"key": "Content-Type", "value": "application/json"}],
                "url": {"raw": f"{{{{baseUrl}}}}/metrics/query/aggregate/{func}"},
                "body": {
                    "mode": "raw",
                    "raw": '{\n  "query": "cpu_usage{instance=\\"server1\\"}"\n}'
                },
                "description": f"MetricsQL {func} aggregate function"
            }
        }
        postman_items.append(item)

    return postman_items

if __name__ == "__main__":
    items = generate_aggregate_postman_items()
    print(json.dumps(items, indent=2))